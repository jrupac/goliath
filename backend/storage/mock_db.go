package storage

import (
	"database/sql"
	"errors"
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
	MarkedArticleIds                []int64

	// Sync channels
	GetAllUsersCalled  chan bool
	ProcessItemsCalled chan bool

	// Function overrides
	OnGetArticlesForFeedForUser                    func(u models.User, feedID int64) ([]models.Article, error)
	OnGetArticlesForUser                           func(u models.User, ids []int64) ([]models.Article, error)
	OnMarkArticlesForUser                          func(u models.User, ids []int64, mark models.MarkAction) (int64, error)
	OnUpdateArticleParsedContentForUser            func(u models.User, articleID int64, parsed string) error
	OnGetAllUsers                                  func() ([]models.User, error)
	OnGetAllFeedsForUser                           func(u models.User) ([]models.Feed, error)
	OnGetAllRetrievalCaches                        func() (map[UserFeedKey]string, error)
	OnGetActiveFeedKeys                            func() (map[UserFeedKey]bool, error)
	OnUpdateEstimatedRefreshIntervalForFeedForUser func(u models.User, folderId, id int64, interval int) error
	OnGetUserByUsername                            func(username string) (models.User, error)
	OnUpdateUserCredentials                        func(u models.User, hashPass, key models.Secret) error
	OnGetArticleContentsForUser                    func(u models.User, afterID int64, limit int) ([]models.Article, error)
	OnUpdateArticleContentForUser                  func(u models.User, id int64, summary, content string) error
	OnCreateSession                                func(u models.User, scheme models.AuthScheme, userAgent string) (models.Secret, error)
	OnLookupSession                                func(token models.Secret) (models.User, models.Session, error)
	OnDeleteSessionForUser                         func(u models.User, id models.SessionId) (int64, error)
	OnGetArticleMetaWithFilterForUser              func(u models.User, filter models.StreamFilter, limit int, cursor models.StreamCursor) ([]models.ArticleMeta, error)
	OnGetAllFoldersForUser                         func(u models.User) ([]models.Folder, error)
	OnGetFeedForUser                               func(u models.User, feedID int64) (models.Feed, error)
	OnGetFeedByUrlForUser                          func(u models.User, url string) (models.Feed, error)
	OnGetFolderForUser                             func(u models.User, folderID int64) (models.Folder, error)
	OnGetRootFolderForUser                         func(u models.User) (models.Folder, error)
	OnInsertFeedForUser                            func(u models.User, f models.Feed, folderID int64) (int64, error)
	OnUpdateFolderForFeedForUser                   func(u models.User, feedID, folderID int64) error
	OnUpdateFeedMetadataForUser                    func(u models.User, f models.Feed) error
	OnDeleteFeedForUser                            func(u models.User, feedID, folderID int64) error
	OnInsertFolderForUser                          func(u models.User, f models.Folder, parentID int64) (int64, error)
	OnRenameFolderForUser                          func(u models.User, folderID int64, name string) error
	OnDeleteFolderForUser                          func(u models.User, folderID int64) (int64, error)
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

func (m *MockDB) GetUserByKey(models.Secret) (models.User, error) { return models.User{}, nil }
func (m *MockDB) GetUserByUsername(username string) (models.User, error) {
	if m.OnGetUserByUsername != nil {
		return m.OnGetUserByUsername(username)
	}
	return models.User{}, nil
}

func (m *MockDB) UpdateUserCredentials(u models.User, hashPass models.Secret, key models.Secret) error {
	if m.OnUpdateUserCredentials != nil {
		return m.OnUpdateUserCredentials(u, hashPass, key)
	}
	return nil
}

func (m *MockDB) CreateSession(u models.User, scheme models.AuthScheme, userAgent string) (models.Secret, error) {
	if m.OnCreateSession != nil {
		return m.OnCreateSession(u, scheme, userAgent)
	}
	return "", nil
}

func (m *MockDB) LookupSession(token models.Secret) (models.User, models.Session, error) {
	if m.OnLookupSession != nil {
		return m.OnLookupSession(token)
	}
	return models.User{}, models.Session{}, errors.New("no such session")
}

func (m *MockDB) GetArticleContentsForUser(u models.User, afterID int64, limit int) ([]models.Article, error) {
	if m.OnGetArticleContentsForUser != nil {
		return m.OnGetArticleContentsForUser(u, afterID, limit)
	}
	return nil, nil
}

func (m *MockDB) UpdateArticleContentForUser(u models.User, id int64, summary, content string) error {
	if m.OnUpdateArticleContentForUser != nil {
		return m.OnUpdateArticleContentForUser(u, id, summary, content)
	}
	return nil
}

func (m *MockDB) GetSessionsForUser(models.User) ([]models.Session, error) { return nil, nil }

func (m *MockDB) DeleteSessionForUser(u models.User, id models.SessionId) (int64, error) {
	if m.OnDeleteSessionForUser != nil {
		return m.OnDeleteSessionForUser(u, id)
	}
	return 0, nil
}

func (m *MockDB) DeleteSessionsForUser(models.User) (int64, error)    { return 0, nil }
func (m *MockDB) DeleteExpiredSessions(time.Time) (int64, error)      { return 0, nil }
func (m *MockDB) GetMuteWordsForUser(models.User) ([]string, error)   { return nil, nil }
func (m *MockDB) UpdateMuteWordsForUser(models.User, []string) error  { return nil }
func (m *MockDB) DeleteMuteWordsForUser(models.User, []string) error  { return nil }
func (m *MockDB) GetUnmuteFeedsForUser(models.User) ([]int64, error)  { return nil, nil }
func (m *MockDB) UpdateUnmuteFeedsForUser(models.User, []int64) error { return nil }
func (m *MockDB) DeleteUnmuteFeedsForUser(models.User, []int64) error { return nil }

func (m *MockDB) GetFeedMuteRegexesForUser(models.User) (map[int64][]string, error) { return nil, nil }
func (m *MockDB) GetMuteRegexesForFeedForUser(models.User, int64) ([]string, error) { return nil, nil }
func (m *MockDB) AddMuteRegexForFeedForUser(models.User, int64, string) error       { return nil }
func (m *MockDB) DeleteMuteRegexForFeedForUser(models.User, int64, string) error    { return nil }

func (m *MockDB) GetActiveFeedKeys() (map[UserFeedKey]bool, error) {
	if m.OnGetActiveFeedKeys != nil {
		return m.OnGetActiveFeedKeys()
	}
	return nil, nil
}
func (m *MockDB) GetAllRetrievalCaches() (map[UserFeedKey]string, error) {
	if m.OnGetAllRetrievalCaches != nil {
		return m.OnGetAllRetrievalCaches()
	}
	return nil, nil
}
func (m *MockDB) PersistAllRetrievalCaches(map[UserFeedKey][]byte) error { return nil }
func (m *MockDB) InsertFeedForUser(u models.User, f models.Feed, folderID int64) (int64, error) {
	if m.OnInsertFeedForUser != nil {
		return m.OnInsertFeedForUser(u, f, folderID)
	}
	return 0, nil
}
func (m *MockDB) InsertFolderForUser(u models.User, f models.Folder, parentID int64) (int64, error) {
	if m.OnInsertFolderForUser != nil {
		return m.OnInsertFolderForUser(u, f, parentID)
	}
	return 0, nil
}
func (m *MockDB) RenameFolderForUser(u models.User, folderID int64, name string) error {
	if m.OnRenameFolderForUser != nil {
		return m.OnRenameFolderForUser(u, folderID, name)
	}
	return nil
}
func (m *MockDB) DeleteFolderForUser(u models.User, folderID int64) (int64, error) {
	if m.OnDeleteFolderForUser != nil {
		return m.OnDeleteFolderForUser(u, folderID)
	}
	return 0, nil
}
func (m *MockDB) DeleteArticlesForUser(models.User, time.Time) (int64, error) { return 0, nil }
func (m *MockDB) DeleteArticlesByIdForUser(models.User, []int64) error        { return nil }
func (m *MockDB) DeleteFeedForUser(u models.User, feedID, folderID int64) error {
	if m.OnDeleteFeedForUser != nil {
		return m.OnDeleteFeedForUser(u, feedID, folderID)
	}
	return nil
}
func (m *MockDB) MarkArticleForUser(models.User, int64, models.MarkAction) error {
	return nil
}
func (m *MockDB) MarkArticlesForUser(u models.User, ids []int64, mark models.MarkAction) (int64, error) {
	if m.OnMarkArticlesForUser != nil {
		return m.OnMarkArticlesForUser(u, ids, mark)
	}
	m.MarkedArticleIds = append(m.MarkedArticleIds, ids...)
	return int64(len(ids)), nil
}
func (m *MockDB) MarkFeedForUser(models.User, int64, models.MarkAction) (int64, error) {
	return 0, nil
}
func (m *MockDB) MarkFolderForUser(models.User, int64, models.MarkAction) (int64, error) {
	return 0, nil
}
func (m *MockDB) UpdateLatestTimeForFeedForUser(models.User, int64, int64, time.Time) error {
	return nil
}
func (m *MockDB) UpdateEstimatedRefreshIntervalForFeedForUser(u models.User, folderId, id int64, interval int) error {
	if m.OnUpdateEstimatedRefreshIntervalForFeedForUser != nil {
		return m.OnUpdateEstimatedRefreshIntervalForFeedForUser(u, folderId, id, interval)
	}
	return nil
}
func (m *MockDB) UpdateFolderForFeedForUser(u models.User, feedID, folderID int64) error {
	if m.OnUpdateFolderForFeedForUser != nil {
		return m.OnUpdateFolderForFeedForUser(u, feedID, folderID)
	}
	return nil
}
func (m *MockDB) GetFolderChildrenForUser(models.User, int64) ([]int64, error) {
	return nil, nil
}
func (m *MockDB) GetAllFoldersForUser(u models.User) ([]models.Folder, error) {
	if m.OnGetAllFoldersForUser != nil {
		return m.OnGetAllFoldersForUser(u)
	}
	return nil, nil
}

func (m *MockDB) GetFeedForUser(u models.User, feedID int64) (models.Feed, error) {
	if m.OnGetFeedForUser != nil {
		return m.OnGetFeedForUser(u, feedID)
	}
	return models.Feed{}, sql.ErrNoRows
}

func (m *MockDB) GetFeedByUrlForUser(u models.User, url string) (models.Feed, error) {
	if m.OnGetFeedByUrlForUser != nil {
		return m.OnGetFeedByUrlForUser(u, url)
	}
	return models.Feed{}, sql.ErrNoRows
}

func (m *MockDB) GetFolderForUser(u models.User, folderID int64) (models.Folder, error) {
	if m.OnGetFolderForUser != nil {
		return m.OnGetFolderForUser(u, folderID)
	}
	return models.Folder{}, sql.ErrNoRows
}

func (m *MockDB) GetRootFolderForUser(u models.User) (models.Folder, error) {
	if m.OnGetRootFolderForUser != nil {
		return m.OnGetRootFolderForUser(u)
	}
	return models.Folder{}, sql.ErrNoRows
}

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
func (m *MockDB) GetArticleMetaWithFilterForUser(u models.User, filter models.StreamFilter, limit int, cursor models.StreamCursor) ([]models.ArticleMeta, error) {
	if m.OnGetArticleMetaWithFilterForUser != nil {
		return m.OnGetArticleMetaWithFilterForUser(u, filter, limit, cursor)
	}
	return nil, nil
}
func (m *MockDB) GetArticlesForUser(u models.User, ids []int64) ([]models.Article, error) {
	if m.OnGetArticlesForUser != nil {
		return m.OnGetArticlesForUser(u, ids)
	}
	return nil, nil
}
func (m *MockDB) GetArticlesWithFilterForUser(models.User, models.StreamFilter, int, int64) ([]models.Article, error) {
	return nil, nil
}
func (m *MockDB) ImportOpmlForUser(models.User, *opml.Opml) error { return nil }

// Methods with mock implementations

func (m *MockDB) UpdateFeedMetadataForUser(u models.User, feed models.Feed) error {
	m.UpdateFeedMetadataForUserCalled = true
	if m.OnUpdateFeedMetadataForUser != nil {
		return m.OnUpdateFeedMetadataForUser(u, feed)
	}
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

func (m *MockDB) UpdateArticleParsedContentForUser(u models.User, articleID int64, parsed string) error {
	if m.OnUpdateArticleParsedContentForUser != nil {
		return m.OnUpdateArticleParsedContentForUser(u, articleID, parsed)
	}
	return nil
}
