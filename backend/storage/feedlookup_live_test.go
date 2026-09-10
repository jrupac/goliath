package storage

import (
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/jrupac/goliath/models"
)

// Exercises the single-feed and single-folder lookups against a real
// CockroachDB. Skipped unless GOLIATH_TEST_DB names one. Read-only.
func TestFeedAndFolderLookupsAgainstDatabase(t *testing.T) {
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
	want := feeds[0]

	got, err := crdb.GetFeedForUser(u, want.ID)
	if err != nil {
		t.Fatalf("GetFeedForUser: %v", err)
	}
	if got.ID != want.ID || got.Title != want.Title || got.URL != want.URL || got.FolderID != want.FolderID {
		t.Errorf("GetFeedForUser returned %+v, want %+v", got, want)
	}

	byUrl, err := crdb.GetFeedByUrlForUser(u, want.URL)
	if err != nil {
		t.Fatalf("GetFeedByUrlForUser: %v", err)
	}
	if byUrl.ID != want.ID {
		t.Errorf("GetFeedByUrlForUser returned feed %d, want %d", byUrl.ID, want.ID)
	}

	// A URL nobody is subscribed to is reported as absent, not as an error the
	// caller has to interpret.
	if _, err = crdb.GetFeedByUrlForUser(u, "https://not.subscribed.invalid/feed"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetFeedByUrlForUser for an unknown URL: %v, want sql.ErrNoRows", err)
	}

	folders, err := crdb.GetAllFoldersForUser(u)
	if err != nil || len(folders) == 0 {
		t.Fatalf("GetAllFoldersForUser: %v (%d folders)", err, len(folders))
	}
	folder, err := crdb.GetFolderForUser(u, folders[0].ID)
	if err != nil {
		t.Fatalf("GetFolderForUser: %v", err)
	}
	if folder.ID != folders[0].ID || folder.Name != folders[0].Name {
		t.Errorf("GetFolderForUser returned %+v, want %+v", folder, folders[0])
	}

	// Ownership is part of the lookup, so another user's feed is reported the
	// same way as one that does not exist.
	stranger := models.User{UserId: "00000000-0000-0000-0000-000000000000", Username: "x", Key: "x"}
	if _, err = crdb.GetFeedForUser(stranger, want.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetFeedForUser as another user: %v, want sql.ErrNoRows", err)
	}
	if _, err = crdb.GetFeedByUrlForUser(stranger, want.URL); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetFeedByUrlForUser as another user: %v, want sql.ErrNoRows", err)
	}
	if _, err = crdb.GetFolderForUser(stranger, folders[0].ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetFolderForUser as another user: %v, want sql.ErrNoRows", err)
	}
}
