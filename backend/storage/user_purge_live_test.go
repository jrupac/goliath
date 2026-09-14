package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jrupac/goliath/models"
)

// Deletes, restores and purges a user holding many articles against a real
// CockroachDB, reporting how long each step takes. Skipped unless
// GOLIATH_TEST_DB names a throwaway database, since it writes;
// GOLIATH_PURGE_ARTICLES sets how many articles the user holds.
func TestUserDeletionAgainstDatabase(t *testing.T) {
	dsn := os.Getenv("GOLIATH_TEST_DB")
	if dsn == "" {
		t.Skip("GOLIATH_TEST_DB not set")
	}
	articles := 100000
	if v := os.Getenv("GOLIATH_PURGE_ARTICLES"); v != "" {
		var err error
		if articles, err = strconv.Atoi(v); err != nil {
			t.Fatalf("GOLIATH_PURGE_ARTICLES: %v", err)
		}
	}

	crdb := &Crdb{}
	if err := crdb.Open(dsn); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer crdb.Close()

	add := func(name string) models.User {
		u, err := models.NewUser(name, models.Secret("pw"))
		if err != nil {
			t.Fatal(err)
		}
		if u, err = crdb.InsertUser(u); err != nil {
			t.Fatalf("InsertUser(%s): %v", name, err)
		}
		return u
	}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	victim := add("purge" + suffix)
	bystander := add("keep" + suffix)

	// Articles are made in the database, which is far quicker than inserting
	// them one by one and makes no difference to what is being measured.
	fill := func(u models.User, n int) int64 {
		feed, err := crdb.InsertFeedForUser(u, models.Feed{Title: "t", URL: "http://example.invalid/" + suffix,
			Link: "http://example.invalid"}, 0)
		if err != nil {
			t.Fatalf("InsertFeedForUser: %v", err)
		}
		_, err = crdb.db.Exec(`
			INSERT INTO Article (userid, feed, hash, title, summary, content, link, read, date, retrieved)
			SELECT f.userid, f.id, 'h' || g::STRING, 'title', repeat('x', 1000), repeat('y', 1000),
			       'http://example.invalid/' || g::STRING, g % 3 = 0, now(), now()
			FROM Feed f, generate_series(1, $2) AS g WHERE f.id = $1`, feed, n)
		if err != nil {
			t.Fatalf("filling articles: %v", err)
		}
		return feed
	}
	start := time.Now()
	fill(victim, articles)
	fill(bystander, 100)
	// One statement writing every article leaves its writes for whoever reads
	// them first to finish resolving, at a cost that would otherwise land on
	// whichever step below reads first. Articles stored one per fetch leave no
	// such backlog, so it is paid here.
	count(t, crdb, `SELECT count(*) FROM Article WHERE userid = $1`, victim.UserId)
	t.Logf("filled %d articles in %s", articles, time.Since(start).Round(time.Millisecond))
	if _, err := crdb.CreateSession(victim, models.AuthSchemeGReader, "test"); err != nil {
		t.Fatal(err)
	}

	start = time.Now()
	del, err := crdb.TombstoneUser(victim)
	if err != nil {
		t.Fatalf("TombstoneUser: %v", err)
	}
	t.Logf("deleting a user with %d articles took %s", articles, time.Since(start).Round(time.Millisecond))
	if del.Articles != int64(articles) || del.Feeds != 1 || del.Sessions != 1 {
		t.Errorf("TombstoneUser reported %+v", del)
	}
	if _, err = crdb.GetUserByUsername(victim.Username); err == nil {
		t.Error("a deleted user is still found by name")
	}
	if _, err = crdb.GetUserByKey(victim.Key); err == nil {
		t.Error("a deleted user is still found by key")
	}
	if n := count(t, crdb, `SELECT count(*) FROM Session WHERE userid = $1`, victim.UserId); n != 0 {
		t.Errorf("%d sessions survive deletion", n)
	}
	if _, err = crdb.InsertUser(models.User{Username: victim.Username, Key: models.Secret("k" + suffix)}); !errors.Is(err, ErrUserPendingPurge) {
		t.Errorf("adding a deleted user's name: %v, want ErrUserPendingPurge", err)
	}

	// Past the window, a deleted user is not restored; within it, they are.
	if _, err = crdb.RestoreUser(victim.Username, time.Now().Add(time.Hour)); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("restoring past the window: %v, want sql.ErrNoRows", err)
	}
	if _, err = crdb.RestoreUser(victim.Username, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("RestoreUser: %v", err)
	}
	if _, err = crdb.GetUserByUsername(victim.Username); err != nil {
		t.Errorf("a restored user is not found: %v", err)
	}
	if _, err = crdb.TombstoneUser(victim); err != nil {
		t.Fatalf("TombstoneUser again: %v", err)
	}

	start = time.Now()
	users, purged, err := crdb.PurgeDeletedUsers(time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("PurgeDeletedUsers: %v", err)
	}
	t.Logf("purging %d users and %d articles took %s", users, purged, time.Since(start).Round(time.Millisecond))
	if purged < int64(articles) {
		t.Errorf("purged %d articles, want at least %d", purged, articles)
	}
	for _, table := range []string{"UserTable", "UserPrefs", "Folder", "FolderChildren", "Feed", "Article", "Session"} {
		column := "userid"
		if table == "UserTable" {
			column = "id"
		}
		if n := count(t, crdb, fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s = $1`, table, column), victim.UserId); n != 0 {
			t.Errorf("%d rows in %s survive the purge", n, table)
		}
	}
	if n := count(t, crdb, `SELECT count(*) FROM Article WHERE userid = $1`, bystander.UserId); n != 100 {
		t.Errorf("the bystander has %d articles after the purge, want 100", n)
	}

	if _, err = crdb.TombstoneUser(bystander); err != nil {
		t.Fatal(err)
	}
	if _, _, err = crdb.PurgeDeletedUsers(time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, crdb *Crdb, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := crdb.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}
