package storage

import (
	"flag"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/opml"
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
	Open(string) error
	Close() error
	InsertUser(models.User) error
	GetAllUsers() ([]models.User, error)
	GetUserByKey(string) (models.User, error)
	GetUserByUsername(string) (models.User, error)
	GetAllRetrievalCaches() (map[string]string, error)
	PersistAllRetrievalCaches(map[string][]byte) error
	InsertArticleForUser(models.User, models.Article) error
	InsertFaviconForUser(models.User, int64, int64, string, []byte) error
	InsertFeedForUser(models.User, models.Feed, int64) (int64, error)
	InsertFolderForUser(models.User, models.Folder, int64) (int64, error)
	DeleteArticlesForUser(models.User, time.Time) (int64, error)
	DeleteArticlesByIdForUser(models.User, []int64) error
	DeleteFeedForUser(models.User, int64, int64) error
	MarkArticleForUser(models.User, int64, string) error
	MarkFeedForUser(models.User, int64, string) error
	MarkFolderForUser(models.User, int64, string) error
	UpdateFeedMetadataForUser(models.User, models.Feed) error
	UpdateLatestTimeForFeedForUser(models.User, int64, int64, time.Time) error
	UpdateFolderForFeedForUser(models.User, int64, int64) error
	GetFolderChildrenForUser(models.User, int64) ([]int64, error)
	GetAllFoldersForUser(models.User) ([]models.Folder, error)
	GetAllFeedsForUser(models.User) ([]models.Feed, error)
	GetFeedsInFolderForUser(models.User, int64) ([]models.Feed, error)
	GetFeedsPerFolderForUser(models.User) (map[int64][]int64, error)
	GetFolderFeedTreeForUser(models.User) (*models.Folder, error)
	GetAllFaviconsForUser(models.User) (map[int64]string, error)
	GetUnreadArticleMetaForUser(models.User, int, int64) ([]models.Article, error)
	GetArticlesForUser(models.User, []int64) ([]models.Article, error)
	GetUnreadArticlesForUser(models.User, int, int64) ([]models.Article, error)
	GetArticlesForFeedForUser(models.User, int64) ([]models.Article, error)
	ImportOpmlForUser(models.User, *opml.Opml) error
}

// Open creates a new database instance and returns a pointer to it.
// It takes the path to the database file as an argument.
func Open(dbPath string) (Database, error) {
	db := &Crdb{}
	return db, db.Open(dbPath)
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
