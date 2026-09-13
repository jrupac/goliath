package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jrupac/goliath/models"
)

// Exercises what fetching and unsubscribing may do to a feed concurrently,
// against a real CockroachDB. Skipped unless GOLIATH_TEST_DB names one.
//
// Destructive: it works on a feed it creates, but finishes by purging every
// unsubscribed feed in the database, as the garbage collector does. Point it
// at a throwaway copy.
func TestFeedTombstoneAgainstDatabase(t *testing.T) {
	dsn := os.Getenv("GOLIATH_TEST_DB")
	if dsn == "" {
		t.Skip("GOLIATH_TEST_DB not set")
	}

	crdb := &Crdb{}
	if err := crdb.Open(dsn); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer crdb.Close()

	users, err := crdb.GetAllUsers()
	if err != nil || len(users) == 0 {
		t.Fatalf("GetAllUsers: %v (%d users)", err, len(users))
	}
	u := users[0]
	root, err := crdb.GetRootFolderForUser(u)
	if err != nil {
		t.Fatalf("GetRootFolderForUser: %v", err)
	}

	marker := fmt.Sprintf("tombstone-live-test-%d", time.Now().UnixNano())
	url := fmt.Sprintf("https://example.invalid/%s/feed", marker)
	feed := models.Feed{Title: marker, URL: url, Link: "https://example.invalid/"}
	id, err := crdb.InsertFeedForUser(u, feed, 0)
	if err != nil {
		t.Fatalf("InsertFeedForUser: %v", err)
	}

	article := func(n int, folder int64) models.Article {
		return models.Article{
			FeedID:    id,
			FolderID:  folder,
			Title:     fmt.Sprintf("%s-%d", marker, n),
			Link:      fmt.Sprintf("https://example.invalid/%s/%d", marker, n),
			Date:      time.Now(),
			Retrieved: time.Now(),
		}
	}
	if err = crdb.InsertArticleForUser(u, article(1, root.ID)); err != nil {
		t.Fatalf("InsertArticleForUser: %v", err)
	}

	// A fetch that read the feed before it was moved still names the old
	// folder. Its articles land in the new one.
	folder, err := crdb.InsertFolderForUser(u, models.Folder{Name: marker}, 0)
	if err != nil {
		t.Fatalf("InsertFolderForUser: %v", err)
	}
	defer func() {
		if _, err := crdb.DeleteFolderForUser(u, folder); err != nil {
			t.Errorf("failed to remove folder %d: %v", folder, err)
		}
	}()
	if err = crdb.UpdateFolderForFeedForUser(u, id, folder); err != nil {
		t.Fatalf("UpdateFolderForFeedForUser: %v", err)
	}
	if err = crdb.InsertArticleForUser(u, article(2, root.ID)); err != nil {
		t.Fatalf("InsertArticleForUser naming the folder the feed left: %v", err)
	}
	articles, err := crdb.GetArticlesForFeedForUser(u, id)
	if err != nil || len(articles) != 2 {
		t.Fatalf("GetArticlesForFeedForUser: %d articles, %v; want 2", len(articles), err)
	}
	var ids []int64
	for _, a := range articles {
		ids = append(ids, a.ID)
		if a.FolderID != folder {
			t.Errorf("article %d is in folder %d, want the feed's folder %d", a.ID, a.FolderID, folder)
		}
	}

	// A fetch that read the feed before it was renamed refreshes its metadata
	// from that copy. The user's title stays.
	stale, err := crdb.GetFeedForUser(u, id)
	if err != nil {
		t.Fatalf("GetFeedForUser: %v", err)
	}
	renamed := stale
	renamed.Title = marker + " renamed"
	if err = crdb.RenameFeedForUser(u, renamed); err != nil {
		t.Fatalf("RenameFeedForUser: %v", err)
	}
	stale.Title = marker + " from the feed"
	stale.Description = "refreshed"
	if err = crdb.UpdateFeedMetadataForUser(u, stale); err != nil {
		t.Fatalf("UpdateFeedMetadataForUser: %v", err)
	}
	if f, err := crdb.GetFeedForUser(u, id); err != nil || f.Title != renamed.Title || !f.TitleOverridden || f.Description != "refreshed" {
		t.Errorf("after a stale metadata refresh: %+v, %v; want the user's title kept and the rest refreshed", f, err)
	}

	// Unsubscribed from: gone from every read a client can make.
	if err = crdb.TombstoneFeedForUser(u, id); err != nil {
		t.Fatalf("TombstoneFeedForUser: %v", err)
	}
	if err = crdb.TombstoneFeedForUser(u, id); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("second tombstone: %v, want sql.ErrNoRows", err)
	}
	checkGone(t, crdb, u, id, url, ids)

	// A fetch still in flight writes nothing into it.
	if err = crdb.InsertArticleForUser(u, article(3, folder)); !errors.Is(err, ErrFeedGone) {
		t.Errorf("InsertArticleForUser into a tombstoned feed: %v, want ErrFeedGone", err)
	}
	if err = crdb.InsertFaviconForUser(u, id, "image/png", []byte{1}); !errors.Is(err, ErrFeedGone) {
		t.Errorf("InsertFaviconForUser into a tombstoned feed: %v, want ErrFeedGone", err)
	}
	if err = crdb.UpdateLatestTimeForFeedForUser(u, id, time.Now()); err != nil {
		t.Errorf("UpdateLatestTimeForFeedForUser: %v", err)
	}
	if n, _ := crdb.GetArticlesForFeedForUser(u, id); len(n) != 2 {
		t.Errorf("feed holds %d articles after writes into its tombstone, want 2", len(n))
	}

	// A tombstoned feed keeps its retrieval cache, since it can come back, but
	// is not fetched.
	active, err := crdb.GetActiveFeedKeys()
	if err != nil || !active[UserFeedKey{UserID: u.UserId, FeedID: id}] {
		t.Errorf("GetActiveFeedKeys left out a tombstoned feed (%v)", err)
	}
	live, err := crdb.GetLiveFeedKeys()
	if err != nil || live[UserFeedKey{UserID: u.UserId, FeedID: id}] {
		t.Errorf("GetLiveFeedKeys listed a tombstoned feed (%v)", err)
	}

	// Removing the folder it is in moves it without counting it.
	other, err := crdb.InsertFolderForUser(u, models.Folder{Name: marker + " other"}, 0)
	if err != nil {
		t.Fatalf("InsertFolderForUser: %v", err)
	}
	if moved, err := crdb.DeleteFolderForUser(u, folder); err != nil || moved != 0 {
		t.Errorf("DeleteFolderForUser: moved %d, %v; want 0, since the only feed is tombstoned", moved, err)
	}
	folder = other

	// Re-adding restores it, articles and all.
	restored, err := crdb.RestoreFeedByUrlForUser(u, url)
	if err != nil || restored.ID != id || restored.FolderID != root.ID {
		t.Fatalf("RestoreFeedByUrlForUser: %+v, %v; want feed %d in the root %d", restored, err, id, root.ID)
	}
	checkVisible(t, crdb, u, id, ids)
	if _, err = crdb.RestoreFeedByUrlForUser(u, url); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("restoring a live feed: %v, want sql.ErrNoRows", err)
	}

	// So does an insert that collides with it, as OPML import and the admin
	// service make.
	if err = crdb.TombstoneFeedForUser(u, id); err != nil {
		t.Fatalf("TombstoneFeedForUser: %v", err)
	}
	if again, err := crdb.InsertFeedForUser(u, renamed, 0); err != nil || again != id {
		t.Errorf("InsertFeedForUser of a tombstoned feed = %d, %v; want %d restored", again, err, id)
	}
	checkVisible(t, crdb, u, id, ids)

	// The garbage collector removes it and its articles for good.
	if err = crdb.TombstoneFeedForUser(u, id); err != nil {
		t.Fatalf("TombstoneFeedForUser: %v", err)
	}
	feeds, purged, err := crdb.PurgeDeletedFeeds(time.Now().Add(time.Minute))
	if err != nil || feeds < 1 || purged < 2 {
		t.Errorf("PurgeDeletedFeeds = %d feeds, %d articles, %v; want at least 1 and 2", feeds, purged, err)
	}
	if n, _ := crdb.GetArticlesForFeedForUser(u, id); len(n) != 0 {
		t.Errorf("%d articles survived the purge", len(n))
	}
	if _, err = crdb.RestoreFeedByUrlForUser(u, url); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("restoring a purged feed: %v, want sql.ErrNoRows", err)
	}
	if active, _ = crdb.GetActiveFeedKeys(); active[UserFeedKey{UserID: u.UserId, FeedID: id}] {
		t.Error("a purged feed is still listed")
	}
}

// checkGone asserts that a feed and its articles are absent from every read a
// client can make.
func checkGone(t *testing.T, crdb *Crdb, u models.User, id int64, url string, articles []int64) {
	t.Helper()
	if _, err := crdb.GetFeedForUser(u, id); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetFeedForUser: %v, want sql.ErrNoRows", err)
	}
	if _, err := crdb.GetFeedByUrlForUser(u, url); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetFeedByUrlForUser: %v, want sql.ErrNoRows", err)
	}
	feeds, _ := crdb.GetAllFeedsForUser(u)
	if slices.ContainsFunc(feeds, func(f models.Feed) bool { return f.ID == id }) {
		t.Error("GetAllFeedsForUser lists a tombstoned feed")
	}
	perFolder, _ := crdb.GetFeedsPerFolderForUser(u)
	for _, ids := range perFolder {
		if slices.Contains(ids, id) {
			t.Error("GetFeedsPerFolderForUser lists a tombstoned feed")
		}
	}
	tree, err := crdb.GetFolderFeedTreeForUser(u)
	if err != nil {
		t.Errorf("GetFolderFeedTreeForUser: %v", err)
	} else if treeHasFeed(*tree, id) {
		t.Error("the folder tree holds a tombstoned feed")
	}
	if got := visibleArticles(t, crdb, u, id, articles); got != 0 {
		t.Errorf("%d of a tombstoned feed's articles are still visible", got)
	}
}

// checkVisible asserts that a feed and all its articles are visible.
func checkVisible(t *testing.T, crdb *Crdb, u models.User, id int64, articles []int64) {
	t.Helper()
	if _, err := crdb.GetFeedForUser(u, id); err != nil {
		t.Errorf("GetFeedForUser: %v", err)
	}
	if got, want := visibleArticles(t, crdb, u, id, articles), 3*len(articles); got != want {
		t.Errorf("%d article sightings, want %d", got, want)
	}
}

// visibleArticles counts sightings of the given articles across the three
// ways a client reads articles: stream metadata, stream contents, and by ID.
func visibleArticles(t *testing.T, crdb *Crdb, u models.User, feedID int64, ids []int64) int {
	t.Helper()
	n := 0
	meta, err := crdb.GetArticleMetaWithFilterForUser(u, models.StreamFilterUnread, -1, models.StreamCursor{SinceID: -1})
	if err != nil {
		t.Fatalf("GetArticleMetaWithFilterForUser: %v", err)
	}
	for _, a := range meta {
		if a.FeedID == feedID {
			n++
		}
	}
	full, err := crdb.GetArticlesWithFilterForUser(u, models.StreamFilterUnread, -1, -1)
	if err != nil {
		t.Fatalf("GetArticlesWithFilterForUser: %v", err)
	}
	for _, a := range full {
		if a.FeedID == feedID {
			n++
		}
	}
	byId, err := crdb.GetArticlesForUser(u, ids)
	if err != nil {
		t.Fatalf("GetArticlesForUser: %v", err)
	}
	return n + len(byId)
}

func treeHasFeed(f models.Folder, id int64) bool {
	if slices.ContainsFunc(f.Feed, func(feed models.Feed) bool { return feed.ID == id }) {
		return true
	}
	return slices.ContainsFunc(f.Folders, func(c models.Folder) bool { return treeHasFeed(c, id) })
}
