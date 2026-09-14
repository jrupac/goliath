package storage

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jrupac/goliath/models"
)

// Exercises the batched article insert against a real CockroachDB: splitting
// by count and by size, what comes back, and a repeat. Skipped unless
// GOLIATH_TEST_DB names one. It removes the feed it creates by purging
// tombstoned feeds, so it wants a throwaway database.
func TestInsertArticlesAgainstDatabase(t *testing.T) {
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

	marker := fmt.Sprintf("batch-live-test-%d", time.Now().UnixNano())
	id, err := crdb.InsertFeedForUser(u, models.Feed{
		Title: marker, URL: fmt.Sprintf("https://example.invalid/%s/feed", marker), Link: "https://example.invalid/",
	}, 0)
	if err != nil {
		t.Fatalf("InsertFeedForUser: %v", err)
	}
	defer func() {
		_ = crdb.TombstoneFeedForUser(u, id)
		if _, _, err := crdb.PurgeDeletedFeeds(time.Now().Add(time.Minute)); err != nil {
			t.Errorf("PurgeDeletedFeeds: %v", err)
		}
	}()
	feed, err := crdb.GetFeedForUser(u, id)
	if err != nil {
		t.Fatalf("GetFeedForUser: %v", err)
	}

	article := func(name, content string, date time.Time, read bool) models.Article {
		return models.Article{
			FeedID:    id,
			Title:     fmt.Sprintf("%s-%s", marker, name),
			Link:      fmt.Sprintf("https://example.invalid/%s/%s", marker, name),
			Content:   content,
			Date:      date,
			Retrieved: date,
			Read:      read,
		}
	}

	// More than one statement's worth by count, dated to the nanosecond.
	base := time.Date(2025, 10, 12, 10, 0, 0, 123456789, time.UTC)
	var many []models.Article
	for i := range 2*maxArticleBatchRows + 50 {
		many = append(many, article(fmt.Sprint(i), "<p>body</p>", base.Add(time.Duration(i)*time.Minute), i%2 == 0))
	}
	if n, err := crdb.InsertArticlesForUser(u, id, many); err != nil || n != len(many) {
		t.Fatalf("InsertArticlesForUser: %d new, %v; want %d", n, err, len(many))
	}
	if n, err := crdb.InsertArticlesForUser(u, id, many); err != nil || n != 0 {
		t.Errorf("inserting them again: %d new, %v; want 0", n, err)
	}

	// More than one statement's worth by size.
	body := strings.Repeat("x", 3<<20)
	var large []models.Article
	for i := range 3 {
		large = append(large, article(fmt.Sprintf("large-%d", i), body, base, false))
	}
	if n, err := crdb.InsertArticlesForUser(u, id, large); err != nil || n != len(large) {
		t.Fatalf("InsertArticlesForUser, large: %d new, %v; want %d", n, err, len(large))
	}

	type row struct {
		read    bool
		date    time.Time
		folder  int64
		content int
	}
	rows, err := crdb.db.Query(
		`SELECT title, read, date, folder, length(content) FROM Article WHERE userid = $1 AND feed = $2`, u.UserId, id)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	got := map[string]row{}
	for rows.Next() {
		var title string
		var r row
		if err = rows.Scan(&title, &r.read, &r.date, &r.folder, &r.content); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		got[title] = r
	}
	closeSilent(rows)
	if len(got) != len(many)+len(large) {
		t.Fatalf("feed holds %d articles, want %d", len(got), len(many)+len(large))
	}
	for _, a := range append(many, large...) {
		r, ok := got[a.Title]
		if !ok {
			t.Errorf("%s was not stored", a.Title)
			continue
		}
		if d := r.date.Sub(a.Date); d < -time.Microsecond || d > time.Microsecond {
			t.Errorf("%s dated %s, want %s", a.Title, r.date, a.Date)
		}
		if r.read != a.Read || r.folder != feed.FolderID || r.content != len(a.Content) {
			t.Errorf("%s stored as %+v, want read=%t in folder %d with %d bytes",
				a.Title, r, a.Read, feed.FolderID, len(a.Content))
		}
	}

	if err = crdb.TombstoneFeedForUser(u, id); err != nil {
		t.Fatalf("TombstoneFeedForUser: %v", err)
	}
	if _, err = crdb.InsertArticlesForUser(u, id, []models.Article{article("late", "", base, false)}); !errors.Is(err, ErrFeedGone) {
		t.Errorf("inserting into a tombstoned feed: %v, want ErrFeedGone", err)
	}
}
