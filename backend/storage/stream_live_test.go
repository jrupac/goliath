package storage

import (
	"os"
	"testing"
	"time"

	"github.com/jrupac/goliath/models"
)

// Exercises feed and folder streams against a real CockroachDB. Skipped unless
// GOLIATH_TEST_DB names one. It only reads.
func TestStreamScopesAgainstDatabase(t *testing.T) {
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

	// A feed with both read and unread articles, so that leaving the read ones
	// out is observable.
	feeds, err := crdb.GetAllFeedsForUser(u)
	if err != nil {
		t.Fatalf("GetAllFeedsForUser: %v", err)
	}
	var feed models.Feed
	var all, unread int
	for _, f := range feeds {
		articles, err := crdb.GetArticlesForFeedForUser(u, f.ID)
		if err != nil {
			t.Fatalf("GetArticlesForFeedForUser: %v", err)
		}
		n := 0
		for _, a := range articles {
			if !a.Read {
				n++
			}
		}
		if n > 0 && n < len(articles) && len(articles) <= MaxFetchedRows {
			feed, all, unread = f, len(articles), n
			break
		}
	}
	if all == 0 {
		t.Skip("no feed holds both read and unread articles")
	}

	read := func(s models.Stream, cursor models.StreamCursor) []models.ArticleMeta {
		t.Helper()
		metas, err := crdb.GetArticleMetaWithFilterForUser(u, s, MaxFetchedRows, cursor)
		if err != nil {
			t.Fatalf("GetArticleMetaWithFilterForUser(%+v): %v", s, err)
		}
		return metas
	}
	inFeed := func(metas []models.ArticleMeta) int {
		n := 0
		for _, m := range metas {
			if m.FeedID == feed.ID {
				n++
			}
		}
		return n
	}

	got := read(models.Stream{Filter: models.StreamFilterAll, FeedID: feed.ID}, models.StreamCursor{})
	if len(got) != all || inFeed(got) != all {
		t.Errorf("feed stream: %d items, %d of them the feed's; want all %d", len(got), inFeed(got), all)
	}

	got = read(models.Stream{Filter: models.StreamFilterAll, FeedID: feed.ID, ExcludeRead: true}, models.StreamCursor{})
	if len(got) != unread || inFeed(got) != unread {
		t.Errorf("feed stream without read items: %d items, %d of them the feed's; want %d", len(got), inFeed(got), unread)
	}

	got = read(models.Stream{Filter: models.StreamFilterAll, FolderID: feed.FolderID}, models.StreamCursor{})
	for _, m := range got {
		if m.FolderID != feed.FolderID {
			t.Errorf("folder stream holds article %d from folder %d, want only %d", m.ID, m.FolderID, feed.FolderID)
			break
		}
	}
	if inFeed(got) != all {
		t.Errorf("folder stream holds %d of the feed's articles, want all %d", inFeed(got), all)
	}

	// The cursor still applies.
	future := models.StreamCursor{Since: time.Now().Add(time.Hour)}
	if got = read(models.Stream{Filter: models.StreamFilterAll, FeedID: feed.ID}, future); len(got) != 0 {
		t.Errorf("feed stream bounded in the future has %d items, want none", len(got))
	}
}
