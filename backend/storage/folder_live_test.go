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

// Exercises creating, renaming and removing a folder against a real
// CockroachDB. Skipped unless GOLIATH_TEST_DB names one.
//
// Destructive: it creates a folder, moves a feed into it and removes it again.
// The feed is put back, but point this at a throwaway copy regardless.
func TestFolderLifecycleAgainstDatabase(t *testing.T) {
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

	// A feed with articles, so that their following it is observable.
	feeds, err := crdb.GetAllFeedsForUser(u)
	if err != nil {
		t.Fatalf("GetAllFeedsForUser: %v", err)
	}
	var feed models.Feed
	var articles int
	for _, f := range feeds {
		a, err := crdb.GetArticlesForFeedForUser(u, f.ID)
		if err != nil {
			t.Fatalf("GetArticlesForFeedForUser: %v", err)
		}
		if len(a) > 0 {
			feed, articles = f, len(a)
			break
		}
	}
	if articles == 0 {
		t.Fatal("no feed has any articles")
	}

	name := fmt.Sprintf("live-test-%d", time.Now().UnixNano())
	id, err := crdb.InsertFolderForUser(u, models.Folder{Name: name}, 0)
	if err != nil {
		t.Fatalf("InsertFolderForUser: %v", err)
	}

	// A folder given no parent is filed under the root, once.
	children, err := crdb.GetFolderChildrenForUser(u, root.ID)
	if err != nil {
		t.Fatalf("GetFolderChildrenForUser: %v", err)
	}
	if n := countOf(children, id); n != 1 {
		t.Errorf("new folder is under the root %d times, want 1", n)
	}

	// Inserting a name that exists merges into it without a second link.
	again, err := crdb.InsertFolderForUser(u, models.Folder{Name: name}, 0)
	if err != nil || again != id {
		t.Errorf("InsertFolderForUser of an existing name = %d, %v; want %d", again, err, id)
	}
	children, _ = crdb.GetFolderChildrenForUser(u, root.ID)
	if n := countOf(children, id); n != 1 {
		t.Errorf("re-inserted folder is under the root %d times, want 1", n)
	}

	if err = crdb.UpdateFolderForFeedForUser(u, feed.ID, id); err != nil {
		t.Fatalf("UpdateFolderForFeedForUser: %v", err)
	}
	// Put back wherever the test stops. By then the feed is in the new folder
	// or, once that is removed, the root; either way it belongs where it was.
	defer func() {
		if err := crdb.UpdateFolderForFeedForUser(u, feed.ID, feed.FolderID); err != nil {
			t.Errorf("failed to put feed %d back in folder %d: %v", feed.ID, feed.FolderID, err)
		}
	}()

	renamed := name + " renamed"
	if err = crdb.RenameFolderForUser(u, id, renamed); err != nil {
		t.Fatalf("RenameFolderForUser: %v", err)
	}
	if f, err := crdb.GetFolderForUser(u, id); err != nil || f.Name != renamed {
		t.Errorf("after rename: %+v, %v; want name %q", f, err, renamed)
	}

	// Onto a name another folder has, including the root's own.
	for _, taken := range []string{models.RootFolder, otherFolderName(t, crdb, u, id)} {
		if err = crdb.RenameFolderForUser(u, id, taken); !errors.Is(err, ErrFolderNameTaken) {
			t.Errorf("rename onto %q: %v, want ErrFolderNameTaken", taken, err)
		}
	}
	if err = crdb.RenameFolderForUser(u, root.ID, "Not the root"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("rename of the root: %v, want sql.ErrNoRows", err)
	}
	if err = crdb.RenameFolderForUser(u, id, models.RootFolderDisplayName); err == nil {
		t.Error("renamed a folder to the root's display name")
	}

	if _, err = crdb.DeleteFolderForUser(u, root.ID); !errors.Is(err, ErrRootFolder) {
		t.Errorf("delete of the root: %v, want ErrRootFolder", err)
	}

	moved, err := crdb.DeleteFolderForUser(u, id)
	if err != nil {
		t.Fatalf("DeleteFolderForUser: %v", err)
	}
	if moved != 1 {
		t.Errorf("moved %d feeds, want 1", moved)
	}
	if _, err = crdb.GetFolderForUser(u, id); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("removed folder still found: %v", err)
	}
	children, _ = crdb.GetFolderChildrenForUser(u, root.ID)
	if countOf(children, id) != 0 {
		t.Error("removed folder is still linked under the root")
	}

	// The feed is now in the root, with every article it had.
	f, err := crdb.GetFeedForUser(u, feed.ID)
	if err != nil || f.FolderID != root.ID {
		t.Errorf("feed after removal: %+v, %v; want it in the root %d", f, err, root.ID)
	}
	a, err := crdb.GetArticlesForFeedForUser(u, feed.ID)
	if err != nil || len(a) != articles {
		t.Errorf("feed has %d articles after removal (%v), want %d", len(a), err, articles)
	}

	if _, err = crdb.DeleteFolderForUser(u, id); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("second removal: %v, want sql.ErrNoRows", err)
	}
}

func countOf(ids []int64, id int64) int {
	n := 0
	for _, x := range ids {
		if x == id {
			n++
		}
	}
	return n
}

// otherFolderName returns the name of one of the user's folders that is neither
// the root nor the one given.
func otherFolderName(t *testing.T, crdb *Crdb, u models.User, not int64) string {
	t.Helper()
	folders, err := crdb.GetAllFoldersForUser(u)
	if err != nil {
		t.Fatalf("GetAllFoldersForUser: %v", err)
	}
	i := slices.IndexFunc(folders, func(f models.Folder) bool {
		return f.ID != not && f.Name != models.RootFolder
	})
	if i < 0 {
		t.Fatal("the user has no other folder")
	}
	return folders[i].Name
}
