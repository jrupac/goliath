package storage

import (
	"flag"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/opml"
	"github.com/jrupac/goliath/schema"
	"github.com/jrupac/goliath/utils"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

var (
	openRetries = flag.Int("openRetries", 5, "Number of retries on opening the DB.")
	pingRetries = flag.Int("pingRetries", 5, "Number of retries on pinging the DB.")
)

var (
	latencyMetric = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "db_op_latency",
			Help:       "Server-side latency of database operations.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"method"},
	)
)

func init() {
	prometheus.MustRegister(latencyMetric)
}

// Database defines an interface for all public methods in the database package.
type Database interface {

	// General

	Open(string) error
	Close() error
	GetSchemaVersions() ([]schema.Applied, error)

	// User management

	InsertUser(models.User) (models.User, error)
	TombstoneUser(models.User) (models.UserDeletion, error)
	RestoreUser(string, time.Time) (models.User, error)
	PurgeDeletedUsers(time.Time) (int64, int64, error)
	GetAllUsers() ([]models.User, error)
	GetUserSummaries() ([]models.UserSummary, error)
	GetUserByKey(models.Secret) (models.User, error)
	GetUserByUsername(string) (models.User, error)
	UpdateUserCredentials(models.User, models.Secret, models.Secret) error

	// Sessions

	CreateSession(models.User, models.AuthScheme, string) (models.Secret, error)
	LookupSession(models.Secret) (models.User, models.Session, error)
	GetSessionsForUser(models.User) ([]models.Session, error)
	DeleteSessionForUser(models.User, models.SessionId) (int64, error)
	DeleteSessionsForUser(models.User) (int64, error)
	DeleteExpiredSessions(time.Time) (int64, error)

	// User preferences

	GetMuteWordsForUser(models.User) ([]string, error)
	UpdateMuteWordsForUser(models.User, []string) error
	DeleteMuteWordsForUser(models.User, []string) error

	GetUnmuteFeedsForUser(models.User) ([]models.FeedId, error)
	UpdateUnmuteFeedsForUser(models.User, []models.FeedId) error
	DeleteUnmuteFeedsForUser(models.User, []models.FeedId) error

	GetFeedMuteRegexesForUser(models.User) (map[models.FeedId][]string, error)
	GetMuteRegexesForFeedForUser(models.User, models.FeedId) ([]string, error)
	AddMuteRegexForFeedForUser(models.User, models.FeedId, string) error
	DeleteMuteRegexForFeedForUser(models.User, models.FeedId, string) error

	// Retrieval cache

	GetActiveFeedKeys() (map[UserFeedKey]bool, error)
	GetLiveFeedKeys() (map[UserFeedKey]bool, error)
	GetAllRetrievalCaches() (map[UserFeedKey]string, error)
	PersistAllRetrievalCaches(map[UserFeedKey][]byte) error

	// Content insertion

	InsertArticlesForUser(models.User, models.FeedId, []models.Article) (int, error)
	InsertFaviconForUser(models.User, models.FeedId, string, []byte) error
	InsertFeedForUser(models.User, models.Feed, models.FolderId) (models.FeedId, error)
	InsertFolderForUser(models.User, models.Folder, models.FolderId) (models.FolderId, error)

	// Content deletion

	DeleteArticlesForUser(models.User, time.Time) (int64, error)
	DeleteArticlesByIdForUser(models.User, []models.ArticleId) error
	TombstoneFeedForUser(models.User, models.FeedId) error
	RestoreFeedByUrlForUser(models.User, string) (models.Feed, error)
	PurgeDeletedFeeds(time.Time) (int64, int64, error)
	DeleteFolderForUser(models.User, models.FolderId) (int64, error)

	// Marking

	MarkArticleForUser(models.User, models.ArticleId, models.MarkAction) error
	MarkArticlesForUser(models.User, []models.ArticleId, models.MarkAction) (int64, error)
	MarkFeedForUser(models.User, models.FeedId, models.MarkAction) (int64, error)
	MarkFolderForUser(models.User, models.FolderId, models.MarkAction) (int64, error)

	// Metadata update

	UpdateFeedMetadataForUser(models.User, models.Feed) error
	RenameFeedForUser(models.User, models.Feed) error
	UpdateLatestTimeForFeedForUser(models.User, models.FeedId, time.Time) error
	UpdateEstimatedRefreshIntervalForFeedForUser(models.User, models.FeedId, int) error
	UpdateFolderForFeedForUser(models.User, models.FeedId, models.FolderId) error
	RenameFolderForUser(models.User, models.FolderId, string) error
	UpdateArticleParsedContentForUser(models.User, models.ArticleId, string) error
	UpdateArticleContentForUser(models.User, models.ArticleId, string, string) error

	// Content retrieval

	GetFolderChildrenForUser(models.User, models.FolderId) ([]models.FolderId, error)
	GetAllFoldersForUser(models.User) ([]models.Folder, error)
	GetAllFeedsForUser(models.User) ([]models.Feed, error)
	GetFeedForUser(models.User, models.FeedId) (models.Feed, error)
	GetFeedByUrlForUser(models.User, string) (models.Feed, error)
	GetFolderForUser(models.User, models.FolderId) (models.Folder, error)
	GetRootFolderForUser(models.User) (models.Folder, error)
	GetFeedsInFolderForUser(models.User, models.FolderId) ([]models.Feed, error)
	GetFeedsPerFolderForUser(models.User) (map[models.FolderId][]models.FeedId, error)
	GetFolderFeedTreeForUser(models.User) (*models.Folder, error)
	GetAllFaviconsForUser(models.User) (map[models.FeedId]string, error)

	GetArticleMetaWithFilterForUser(models.User, models.Stream, int, models.StreamCursor) ([]models.ArticleMeta, error)
	GetArticleContentsForUser(models.User, models.ArticleId, int) ([]models.Article, error)
	GetArticlesForUser(models.User, []models.ArticleId) ([]models.Article, error)
	GetArticlesWithFilterForUser(models.User, models.StreamFilter, int, models.ArticleId) ([]models.Article, error)
	GetArticlesForFeedForUser(models.User, models.FeedId) ([]models.Article, error)

	// OPML

	ImportOpmlForUser(models.User, *opml.Opml) error
}

// Open creates a new database instance and returns a pointer to it.
// It takes the path to the database file as an argument.
func Open(dbPath string) (Database, error) {
	db := &Crdb{}
	return db, db.Open(dbPath)
}

// UserFeedKey is a composite key identifying a specific feed for a specific user.
type UserFeedKey struct {
	UserID models.UserId
	FeedID models.FeedId
}

// int64s converts typed IDs for a driver helper that accepts []int64 but not a
// named integer type, pq.Array among them.
func int64s[T ~int64](ids []T) []int64 {
	out := make([]int64, len(ids))
	for i, id := range ids {
		out[i] = int64(id)
	}
	return out
}

/*******************************************************************************
 * Helper methods
 ******************************************************************************/
func logElapsedTime(t time.Time, method string) {
	utils.Elapsed(t, func(d time.Duration) {
		// Record latency measurements in microseconds.
		latencyMetric.WithLabelValues(method).Observe(float64(d) / float64(time.Microsecond))
		if d > slowOpLogThreshold {
			log.V(2).Infof("Slow operation for method %s: %s", method, d)
		}
	})
}
