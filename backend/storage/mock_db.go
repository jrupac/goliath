package storage

import (
	"time"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/opml"
)

// MockDB is a mock implementation of the Database interface for testing.
// It provides default no-op implementations for all methods.
// Tests can set public fields to control return values and track calls.
type MockDB struct {
	// Errors to return
	UpdateFeedMetadataForUserErr error
	InsertFaviconForUserErr      error

	// Call trackers
	InsertFaviconForUserCalled      bool
	UpdateFeedMetadataForUserCalled bool
	InsertedArticles                []models.Article

	// Sync channels
	GetAllUsersCalled  chan bool
	ProcessItemsCalled chan bool

	// Function overrides
	OnGetArticlesForFeedForUser func(u models.User, feedID int64) ([]models.Article, error)
	OnGetAllUsers               func() ([]models.User, error)
	OnGetAllFeedsForUser        func(u models.User) ([]models.Feed, error)
	OnGetAllRetrievalCaches     func() (map[string]string, error)
}

func (m *MockDB) Open(string) error            { return nil }
func (m *MockDB) Close() error                 { return nil }
func (m *MockDB) InsertUser(models.User) error { return nil }

func (m *MockDB) GetAllUsers() ([]models.User, error) {
	if m.GetAllUsersCalled != nil {
		m.GetAllUsersCalled <- true
	}

	if m.OnGetAllUsers != nil {
		return m.OnGetAllUsers()
	}
	return nil, nil
}

func (m *MockDB) GetUserByKey(string) (models.User, error)            { return models.User{}, nil }
func (m *MockDB) GetUserByUsername(string) (models.User, error)       { return models.User{}, nil }
func (m *MockDB) GetMuteWordsForUser(models.User) ([]string, error)   { return nil, nil }
func (m *MockDB) UpdateMuteWordsForUser(models.User, []string) error  { return nil }
func (m *MockDB) DeleteMuteWordsForUser(models.User, []string) error  { return nil }
func (m *MockDB) GetUnmuteFeedsForUser(models.User) ([]int64, error)  { return nil, nil }
func (m *MockDB) UpdateUnmuteFeedsForUser(models.User, []int64) error { return nil }
func (m *MockDB) DeleteUnmuteFeedsForUser(models.User, []int64) error { return nil }

func (m *MockDB) GetAllRetrievalCaches() (map[string]string, error) {
	if m.OnGetAllRetrievalCaches != nil {
		return m.OnGetAllRetrievalCaches()
	}
	return nil, nil
}
func (m *MockDB) PersistAllRetrievalCaches(map[string][]byte) error { return nil }
func (m *MockDB) InsertFeedForUser(models.User, models.Feed, int64) (int64, error) {
	return 0, nil
}
func (m *MockDB) InsertFolderForUser(models.User, models.Folder, int64) (int64, error) {
	return 0, nil
}
func (m *MockDB) DeleteArticlesForUser(models.User, time.Time) (int64, error) { return 0, nil }
func (m *MockDB) DeleteArticlesByIdForUser(models.User, []int64) error        { return nil }
func (m *MockDB) DeleteFeedForUser(models.User, int64, int64) error           { return nil }
func (m *MockDB) MarkArticleForUser(models.User, int64, models.MarkAction) error {
	return nil
}
func (m *MockDB) MarkFeedForUser(models.User, int64, models.MarkAction) error   { return nil }
func (m *MockDB) MarkFolderForUser(models.User, int64, models.MarkAction) error { return nil }
func (m *MockDB) UpdateLatestTimeForFeedForUser(models.User, int64, int64, time.Time) error {
	return nil
}
func (m *MockDB) UpdateFolderForFeedForUser(models.User, int64, int64) error { return nil }
func (m *MockDB) GetFolderChildrenForUser(models.User, int64) ([]int64, error) {
	return nil, nil
}
func (m *MockDB) GetAllFoldersForUser(models.User) ([]models.Folder, error) { return nil, nil }

func (m *MockDB) GetAllFeedsForUser(u models.User) ([]models.Feed, error) {
	if m.OnGetAllFeedsForUser != nil {
		return m.OnGetAllFeedsForUser(u)
	}
	return nil, nil
}

func (m *MockDB) GetFeedsInFolderForUser(models.User, int64) ([]models.Feed, error) {
	return nil, nil
}
func (m *MockDB) GetFeedsPerFolderForUser(models.User) (map[int64][]int64, error) {
	return nil, nil
}
func (m *MockDB) GetFolderFeedTreeForUser(models.User) (*models.Folder, error) {
	return nil, nil
}
func (m *MockDB) GetAllFaviconsForUser(models.User) (map[int64]string, error) {
	return nil, nil
}
func (m *MockDB) GetArticleMetaWithFilterForUser(models.User, models.StreamFilter, int, int64) ([]models.ArticleMeta, error) {
	return nil, nil
}
func (m *MockDB) GetArticlesForUser(models.User, []int64) ([]models.Article, error) {
	return nil, nil
}
func (m *MockDB) GetArticlesWithFilterForUser(models.User, models.StreamFilter, int, int64) ([]models.Article, error) {
	return nil, nil
}
func (m *MockDB) ImportOpmlForUser(models.User, *opml.Opml) error { return nil }

// Methods with mock implementations

func (m *MockDB) UpdateFeedMetadataForUser(u models.User, feed models.Feed) error {
	m.UpdateFeedMetadataForUserCalled = true
	return m.UpdateFeedMetadataForUserErr
}

func (m *MockDB) InsertFaviconForUser(u models.User, folderId, id int64, mime string, favicon []byte) error {
	m.InsertFaviconForUserCalled = true
	return m.InsertFaviconForUserErr
}

func (m *MockDB) InsertArticleForUser(u models.User, a models.Article) error {
	m.InsertedArticles = append(m.InsertedArticles, a)
	if m.ProcessItemsCalled != nil {
		m.ProcessItemsCalled <- true
	}
	return nil
}

func (m *MockDB) GetArticlesForFeedForUser(u models.User, feedID int64) ([]models.Article, error) {
	if m.OnGetArticlesForFeedForUser != nil {
		return m.OnGetArticlesForFeedForUser(u, feedID)
	}
	return []models.Article{}, nil
}
