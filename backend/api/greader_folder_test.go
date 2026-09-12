package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
)

const (
	renameTagPath  = "/greader/reader/api/0/rename-tag"
	disableTagPath = "/greader/reader/api/0/disable-tag"
)

// folderDB is a mock holding the given folders, answering lookups by ID the way
// the real one does: only for folders in the list.
func folderDB(folders ...models.Folder) *storage.MockDB {
	return &storage.MockDB{
		OnGetAllFoldersForUser: func(models.User) ([]models.Folder, error) {
			return slices.Clone(folders), nil
		},
		OnGetFolderForUser: func(_ models.User, id int64) (models.Folder, error) {
			for _, f := range folders {
				if f.ID == id {
					return f, nil
				}
			}
			return models.Folder{}, sql.ErrNoRows
		},
	}
}

var (
	testRoot = models.Folder{ID: 1, Name: models.RootFolder}
	testOld  = models.Folder{ID: 3, Name: "Old"}
	testTech = models.Folder{ID: 4, Name: "Tech"}
)

// The root is listed under the name subscription/list gives it, so that the
// category a client sees on a feed is a tag it can also find here.
func TestTagListPresentsTheRootUnderItsDisplayName(t *testing.T) {
	mockDB := folderDB(testTech, testRoot)
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleTagList(w, httptest.NewRequest("GET", "/greader/reader/api/0/tag/list", nil), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	var res greaderTagList
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := []greaderTag{
		{Id: starredStreamId},
		{Id: "user/-/label/1", Label: models.RootFolderDisplayName, Type: "folder"},
		{Id: "user/-/label/4", Label: "Tech", Type: "folder"},
	}
	if !slices.Equal(res.Tags, want) {
		t.Errorf("tags = %+v, want %+v", res.Tags, want)
	}
}

// moveDB is a folder mock holding feed 7 in folder 3, recording where a move
// sends it and what folder, if any, is created on the way.
func moveDB(t *testing.T, toFolder *int64, created *models.Folder, folders ...models.Folder) *storage.MockDB {
	t.Helper()
	mockDB := folderDB(folders...)
	mockDB.OnGetFeedForUser = func(models.User, int64) (models.Feed, error) {
		return models.Feed{ID: 7, FolderID: 3}, nil
	}
	mockDB.OnUpdateFolderForFeedForUser = func(_ models.User, _, folderID int64) error {
		*toFolder = folderID
		return nil
	}
	mockDB.OnInsertFolderForUser = func(_ models.User, f models.Folder, parentID int64) (int64, error) {
		if created == nil {
			t.Errorf("created folder %q, which already exists", f.Name)
			return 0, nil
		}
		if parentID != 0 {
			t.Errorf("created folder under %d, want the root", parentID)
		}
		*created = f
		return 40, nil
	}
	return mockDB
}

// A client makes a folder by filing a feed under a label that does not exist
// yet; there is no request that only creates one.
func TestSubscriptionEditCreatesAFolderNamedByLabel(t *testing.T) {
	var toFolder int64
	var created models.Folder
	mockDB := moveDB(t, &toFolder, &created, testRoot, testOld)
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w, editRequest(user, url.Values{
		"ac": {"edit"}, "s": {"feed/7"}, "a": {"user/-/label/ New Folder "},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	if created.Name != "New Folder" {
		t.Errorf("created folder %q, want %q", created.Name, "New Folder")
	}
	if toFolder != 40 {
		t.Errorf("moved to folder %d, want the new folder 40", toFolder)
	}
}

func TestSubscriptionEditFilesUnderAnExistingFolderByName(t *testing.T) {
	var toFolder int64
	mockDB := moveDB(t, &toFolder, nil, testRoot, testOld, testTech)
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w, editRequest(user, url.Values{
		"ac": {"edit"}, "s": {"feed/7"}, "a": {"user/-/label/Tech"},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	if toFolder != testTech.ID {
		t.Errorf("moved to folder %d, want %d", toFolder, testTech.ID)
	}
}

// The root is shown as "Uncategorized", so a client filing a feed under that
// name is asking for the root, not for a new folder that the name is reserved
// against anyway.
func TestSubscriptionEditUncategorizedMeansTheRoot(t *testing.T) {
	var toFolder int64
	mockDB := moveDB(t, &toFolder, nil, testRoot, testOld)
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w, editRequest(user, url.Values{
		"ac": {"edit"}, "s": {"feed/7"}, "a": {"user/-/label/uncategorized"},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	if toFolder != testRoot.ID {
		t.Errorf("moved to folder %d, want the root %d", toFolder, testRoot.ID)
	}
}

// Taken as a name, another kind of stream ID would make a folder called
// "user/-/state/...".
func TestSubscriptionEditRefusesAStateAsAFolder(t *testing.T) {
	var toFolder int64
	mockDB := moveDB(t, &toFolder, nil, testRoot, testOld)
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w, editRequest(user, url.Values{
		"ac": {"edit"}, "s": {"feed/7"}, "a": {starredStreamId},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", got, http.StatusBadRequest)
	}
	if toFolder != 0 {
		t.Errorf("moved to folder %d, want no move", toFolder)
	}
}

// renameDB is a folder mock recording what a rename asked for.
func renameDB(renamed *models.Folder, folders ...models.Folder) *storage.MockDB {
	mockDB := folderDB(folders...)
	mockDB.OnRenameFolderForUser = func(_ models.User, id int64, name string) error {
		*renamed = models.Folder{ID: id, Name: name}
		return nil
	}
	return mockDB
}

func TestRenameTagRenamesByIdOrName(t *testing.T) {
	for _, s := range []string{"user/-/label/4", "4", "user/-/label/Tech"} {
		var renamed models.Folder
		mockDB := renameDB(&renamed, testRoot, testTech)
		user := models.User{UserId: "u", Username: "u"}
		w := httptest.NewRecorder()
		GReader{d: mockDB}.handleRenameTag(w, subscriptionRequest(user, renameTagPath,
			url.Values{"s": {s}, "dest": {"user/-/label/Technology"}}), user)

		if got := w.Result().StatusCode; got != http.StatusOK {
			t.Errorf("s=%q: status = %d, want %d", s, got, http.StatusOK)
		}
		if want := (models.Folder{ID: 4, Name: "Technology"}); renamed.ID != want.ID || renamed.Name != want.Name {
			t.Errorf("s=%q: renamed %+v, want %+v", s, renamed, want)
		}
	}
}

// Every one of these must leave the folder alone.
func TestRenameTagRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		form url.Values
		want int
	}{
		// Its stored name is how the root is found.
		{"the root", url.Values{"s": {"user/-/label/1"}, "dest": {"user/-/label/Mine"}}, http.StatusBadRequest},
		{"the root by display name", url.Values{"s": {"user/-/label/Uncategorized"}, "dest": {"Mine"}}, http.StatusBadRequest},
		// Reserved, so that no folder can be mistaken for the root.
		{"onto the reserved name", url.Values{"s": {"4"}, "dest": {"user/-/label/Uncategorized"}}, http.StatusBadRequest},
		{"onto nothing", url.Values{"s": {"4"}, "dest": {"user/-/label/  "}}, http.StatusBadRequest},
		{"onto a state", url.Values{"s": {"4"}, "dest": {starredStreamId}}, http.StatusBadRequest},
		{"without a source", url.Values{"dest": {"Mine"}}, http.StatusBadRequest},
		{"another user's folder", url.Values{"s": {"user/-/label/99"}, "dest": {"Mine"}}, http.StatusNotFound},
		{"a folder that does not exist", url.Values{"s": {"user/-/label/Nope"}, "dest": {"Mine"}}, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := folderDB(testRoot, testTech)
			mockDB.OnRenameFolderForUser = func(models.User, int64, string) error {
				t.Error("renamed a folder")
				return nil
			}
			user := models.User{UserId: "u", Username: "u"}
			w := httptest.NewRecorder()
			GReader{d: mockDB}.handleRenameTag(w, subscriptionRequest(user, renameTagPath, tc.form), user)

			if got := w.Result().StatusCode; got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

// Names are unique per user. The storage layer is what knows a name is taken,
// since checking first would race another rename.
func TestRenameTagReportsATakenNameAsAConflict(t *testing.T) {
	mockDB := folderDB(testRoot, testOld, testTech)
	mockDB.OnRenameFolderForUser = func(models.User, int64, string) error {
		return storage.ErrFolderNameTaken
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleRenameTag(w, subscriptionRequest(user, renameTagPath,
		url.Values{"s": {"4"}, "dest": {"Old"}}), user)

	if got := w.Result().StatusCode; got != http.StatusConflict {
		t.Errorf("status = %d, want %d", got, http.StatusConflict)
	}
}

// disableDB is a folder mock recording which folders were removed.
func disableDB(t *testing.T, removed *[]int64, folders ...models.Folder) *storage.MockDB {
	t.Helper()
	mockDB := folderDB(folders...)
	mockDB.OnDeleteFolderForUser = func(_ models.User, id int64) (int64, error) {
		if removed == nil {
			t.Errorf("removed folder %d", id)
			return 0, nil
		}
		*removed = append(*removed, id)
		return 2, nil
	}
	return mockDB
}

func TestDisableTagRemovesEveryFolderNamed(t *testing.T) {
	var removed []int64
	mockDB := disableDB(t, &removed, testRoot, testOld, testTech)
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleDisableTag(w, subscriptionRequest(user, disableTagPath,
		url.Values{"s": {"user/-/label/3", "user/-/label/Tech", "3"}}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	// Folder 3 was named twice, and is removed once.
	if want := []int64{3, 4}; !slices.Equal(removed, want) {
		t.Errorf("removed %v, want %v", removed, want)
	}
}

// Each of these must remove nothing, including the folders named before the
// one that is refused.
func TestDisableTagRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		form url.Values
		want int
	}{
		{"the root", url.Values{"s": {"user/-/label/3", "user/-/label/1"}}, http.StatusBadRequest},
		{"another user's folder", url.Values{"s": {"user/-/label/3", "user/-/label/99"}}, http.StatusNotFound},
		{"a folder that does not exist", url.Values{"s": {"user/-/label/3"}, "t": {"Nope"}}, http.StatusNotFound},
		{"nothing", url.Values{}, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := disableDB(t, nil, testRoot, testOld)
			user := models.User{UserId: "u", Username: "u"}
			w := httptest.NewRecorder()
			GReader{d: mockDB}.handleDisableTag(w, subscriptionRequest(user, disableTagPath, tc.form), user)

			if got := w.Result().StatusCode; got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

// Both change or destroy folders, so neither may run on an unsigned request.
func TestFolderEndpointsRequireAPostToken(t *testing.T) {
	mockDB := disableDB(t, nil, testRoot, testOld)
	mockDB.OnRenameFolderForUser = func(models.User, int64, string) error {
		t.Error("renamed a folder without a post token")
		return nil
	}
	user := models.User{UserId: "u", Username: "u"}

	for _, tc := range []struct {
		name    string
		path    string
		form    url.Values
		handler func(http.ResponseWriter, *http.Request, models.User)
	}{
		{"rename-tag", renameTagPath, url.Values{"s": {"3"}, "dest": {"New"}}, GReader{d: mockDB}.handleRenameTag},
		{"disable-tag", disableTagPath, url.Values{"s": {"3"}}, GReader{d: mockDB}.handleDisableTag},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := subscriptionRequest(user, tc.path, tc.form)
			req.Form.Del(postTokenParam)

			w := httptest.NewRecorder()
			tc.handler(w, req, user)

			if got := w.Result().StatusCode; got != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", got, http.StatusUnauthorized)
			}
		})
	}
}
