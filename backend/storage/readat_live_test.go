package storage

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jrupac/goliath/models"
)

// Exercises the read-time bookkeeping and the stream cursor against a real
// CockroachDB. Skipped unless GOLIATH_TEST_DB names one, since it writes.
//
// The articles it inserts are removed on the way out, and it never touches
// articles it did not create.
func TestReadAtAgainstDatabase(t *testing.T) {
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

	feeds, err := crdb.GetAllFeedsForUser(u)
	if err != nil || len(feeds) == 0 {
		t.Fatalf("GetAllFeedsForUser: %v (%d feeds)", err, len(feeds))
	}
	feed := feeds[0]

	// Titles are unique per run so that a previous run's leftovers, if any,
	// cannot be mistaken for this run's articles.
	marker := fmt.Sprintf("readat-live-test-%d", time.Now().UnixNano())
	const count = 3
	for i := 0; i < count; i++ {
		a := models.Article{
			FeedID:    feed.ID,
			FolderID:  feed.FolderID,
			Title:     fmt.Sprintf("%s-%d", marker, i),
			Link:      fmt.Sprintf("https://example.invalid/%s/%d", marker, i),
			Date:      time.Now(),
			Retrieved: time.Now(),
		}
		if err = crdb.InsertArticleForUser(u, a); err != nil {
			t.Fatalf("InsertArticleForUser: %v", err)
		}
	}

	var ids []int64
	articles, err := crdb.GetArticlesForFeedForUser(u, feed.ID)
	if err != nil {
		t.Fatalf("GetArticlesForFeedForUser: %v", err)
	}
	for _, a := range articles {
		if len(a.Title) >= len(marker) && a.Title[:len(marker)] == marker {
			ids = append(ids, a.ID)
		}
	}
	defer func() {
		if err := crdb.DeleteArticlesByIdForUser(u, ids); err != nil {
			t.Errorf("cleaning up %d articles: %v", len(ids), err)
		}
	}()
	if len(ids) != count {
		t.Fatalf("found %d inserted articles, want %d", len(ids), count)
	}

	// readAt reports the recorded read time of one article, or the zero time
	// if it has none.
	readAt := func(id int64) time.Time {
		var at *time.Time
		if err := crdb.db.QueryRow(
			`SELECT readat FROM Article WHERE userid = $1 AND id = $2`, u.UserId, id).Scan(&at); err != nil {
			t.Fatalf("reading readat for %d: %v", id, err)
		}
		if at == nil {
			return time.Time{}
		}
		return *at
	}
	// inCursor reports whether the article is in the read stream bounded by
	// `since`.
	inCursor := func(id int64, since time.Time) bool {
		metas, err := crdb.GetArticleMetaWithFilterForUser(
			u, models.StreamFilterRead, MaxFetchedRows, models.StreamCursor{Since: since})
		if err != nil {
			t.Fatalf("GetArticleMetaWithFilterForUser: %v", err)
		}
		for _, m := range metas {
			if m.ID == id {
				return true
			}
		}
		return false
	}

	if got := readAt(ids[0]); !got.IsZero() {
		t.Errorf("a freshly inserted article has readat %s, want none", got)
	}

	before := time.Now()
	if _, err = crdb.MarkArticlesForUser(u, ids, models.MarkActionRead); err != nil {
		t.Fatalf("MarkArticlesForUser: %v", err)
	}
	first := readAt(ids[0])
	if first.Before(before) {
		t.Fatalf("readat = %s, want at or after %s", first, before)
	}

	if !inCursor(ids[0], before.Add(-time.Hour)) {
		t.Error("a just-read article is missing from the read stream")
	}
	// The bound is a time, not an item ID: an article read before the bound is
	// out of the stream even though its ID is far larger than any timestamp.
	if inCursor(ids[0], time.Now().Add(time.Hour)) {
		t.Error("the read stream ignored a time bound in the future")
	}

	// Re-marking a read article is not a new read event, which matters because
	// marking a whole feed read re-marks every article already read in it.
	if _, err = crdb.MarkFeedForUser(u, feed.ID, models.MarkActionRead); err != nil {
		t.Fatalf("MarkFeedForUser: %v", err)
	}
	if got := readAt(ids[0]); !got.Equal(first) {
		t.Errorf("re-marking read moved readat from %s to %s", first, got)
	}

	// Marking unread returns the article to "never read".
	if err = crdb.MarkArticleForUser(u, ids[0], models.MarkActionUnread); err != nil {
		t.Fatalf("MarkArticleForUser: %v", err)
	}
	if got := readAt(ids[0]); !got.IsZero() {
		t.Errorf("readat = %s after marking unread, want none", got)
	}
	if inCursor(ids[0], before.Add(-time.Hour)) {
		t.Error("an article marked unread is still in the read stream")
	}

	// Marking read again records a new time rather than restoring the old one.
	if err = crdb.MarkArticleForUser(u, ids[0], models.MarkActionRead); err != nil {
		t.Fatalf("MarkArticleForUser: %v", err)
	}
	if got := readAt(ids[0]); !got.After(first) {
		t.Errorf("readat = %s after re-reading, want after %s", got, first)
	}
}
