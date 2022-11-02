package storage

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/opml"
	"github.com/jrupac/goliath/utils"
	"github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"

	"strings"
	"time"
)

const (
	dialect            = "postgres"
	maxFetchedRows     = 10000
	slowOpLogThreshold = 50 * time.Millisecond
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

// Database is a wrapper type around a database connection.
type Database struct {
	db *sql.DB
}

func init() {
	prometheus.MustRegister(latencyMetric)
}

// Open opens a connection to the given database path and tests connectivity.
func Open(dbPath string) (*Database, error) {
	db := new(Database)

	d, err := sql.Open(dialect, dbPath)
	if err != nil {
		return nil, err
	}
	if err = d.Ping(); err != nil {
		return nil, err
	}

	db.db = d
	return db, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

/*******************************************************************************
 * User methods
 ******************************************************************************/

// InsertUser inserts the given user into the database.
func (d *Database) InsertUser(u models.User) error {
	defer logElapsedTime(time.Now(), "InsertUser")

	_, err := d.db.Exec(`INSERT INTO UserTable (id, username, key) VALUES($1, $2, $3)`, u.UserId, u.Username, u.Key)

	return err
}

// GetAllUsers returns a list of all models.User objects.
func (d *Database) GetAllUsers() ([]models.User, error) {
	defer logElapsedTime(time.Now(), "GetAllUsers")

	var users []models.User
	rows, err := d.db.Query(`SELECT id, username, key FROM UserTable`)
	if err != nil {
		return users, err
	}
	defer closeSilent(rows)

	for rows.Next() {
		u := models.User{}
		if err = rows.Scan(&u.UserId, &u.Username, &u.Key); err != nil {
			return users, err
		}
		users = append(users, u)
	}

	return users, err
}

// GetUserByKey returns a user identified by the given key.
func (d *Database) GetUserByKey(key string) (models.User, error) {
	defer logElapsedTime(time.Now(), "GetUserByKey")

	var u models.User
	err := d.db.QueryRow(
		`SELECT id, username, key FROM UserTable WHERE key = $1`, key).Scan(
		&u.UserId, &u.Username, &u.Key)
	if !u.Valid() {
		return models.User{}, errors.New("could not find user")
	}

	return u, err
}

// GetUserByUsername returns a user identified by the given username.
func (d *Database) GetUserByUsername(username string) (models.User, error) {
	defer logElapsedTime(time.Now(), "GetUserByUsername")

	var u models.User
	err := d.db.QueryRow(
		`SELECT id, username, key, hashpass FROM UserTable WHERE username = $1`, username).Scan(
		&u.UserId, &u.Username, &u.Key, &u.HashPass)
	if !u.Valid() {
		return models.User{}, errors.New("could not find user")
	}
	return u, err
}

// GetAllRetrievalCaches retrieves the cache for all users.
func (d *Database) GetAllRetrievalCaches() (map[string]string, error) {
	defer logElapsedTime(time.Now(), "GetAllRetrievalCaches")

	rows, err := d.db.Query(`SELECT userid, cache FROM RetrievalCache`)
	if err != nil {
		return nil, err
	}
	defer closeSilent(rows)

	ret := map[string]string{}

	for rows.Next() {
		var id string
		var cache string
		if err = rows.Scan(&id, &cache); err != nil {
			return nil, err
		}
		ret[id] = cache
	}

	return ret, nil
}

// PersistAllRetrievalCaches writes the retrieval caches for all users.
func (d *Database) PersistAllRetrievalCaches(entries map[string][]byte) error {
	defer logElapsedTime(time.Now(), "PersistAllRetrievalCaches")

	var values []string
	for id, cache := range entries {
		values = append(values, fmt.Sprintf("('%s', x'%x'::STRING)", id, cache))
	}
	valueStr := strings.Join(values, ",")

	_, err := d.db.Exec(`UPSERT INTO RetrievalCache(userid, cache) VALUES` + valueStr)

	return err
}

/*******************************************************************************
 * Insertion/deletion methods
 ******************************************************************************/

// InsertArticleForUser inserts the given article object into the database.
func (d *Database) InsertArticleForUser(u models.User, a models.Article) error {
	defer logElapsedTime(time.Now(), "InsertArticleForUser")

	var articleID int64
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM Article WHERE userid = $1 AND hash = $2`,
		u.UserId, a.Hash()).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		log.V(2).Infof("Duplicate article entry, skipping (hash): %s", a.Hash())
		return nil
	}

	err = d.db.QueryRow(
		`INSERT INTO Article
		(userid, feed, folder, hash, title, summary, content, parsed, link, read, date, retrieved)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id`,
		u.UserId, a.FeedID, a.FolderID, a.Hash(), a.Title, a.Summary, a.Content, a.Parsed, a.Link, a.Read, a.Date, a.Retrieved).Scan(&articleID)
	if err != nil {
		return err
	}
	a.ID = articleID
	return nil
}

// InsertFaviconForUser inserts the given favicon and associated metadata into
// the database.
func (d *Database) InsertFaviconForUser(u models.User, folderId int64, feedId int64, mime string, img []byte) error {
	defer logElapsedTime(time.Now(), "InsertFaviconForUser")

	// TODO: Consider wrapping this into a Favicon model type.
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM Feed WHERE userid = $1 AND folder = $2 AND id = $3`,
		u.UserId, folderId, feedId).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("original feed not in table: %d", feedId)
	}

	// Convert to a base64 encoded string before inserting
	h := base64.StdEncoding.EncodeToString(img)

	_, err = d.db.Exec(
		`UPDATE Feed SET favicon = $1, mime = $2 WHERE userid = $3 AND folder = $4 AND id = $5`,
		h, mime, u.UserId, folderId, feedId)
	return err
}

// InsertFeedForUser inserts a new feed into the database. If `folderId` is 0,
// the feed is assumed to be a top-level entry. Otherwise, the feed will be
// nested under the folder with that ID.
func (d *Database) InsertFeedForUser(u models.User, f models.Feed, folderId int64) (int64, error) {
	defer logElapsedTime(time.Now(), "InsertFeedForUser")

	var feedID int64

	// If the feed is assumed to be a top-level entry, determine the ID of the
	// root folder that it actually is under.
	if folderId == 0 {
		err := d.db.QueryRow(`SELECT id FROM Folder WHERE userid = $1 AND name = $2`,
			u.UserId, models.RootFolder).Scan(&folderId)
		if err != nil {
			return feedID, nil
		}
	}

	err := d.db.QueryRow(
		`INSERT INTO Feed(userid, folder, hash, title, description, url, link)
			VALUES($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT(hash) DO UPDATE SET hash = excluded.hash RETURNING id`,
		u.UserId, folderId, f.Hash(), f.Title, f.Description, f.URL, f.Link).Scan(&feedID)
	return feedID, err
}

// InsertFolderForUser inserts a new folder into the database. If `parentId` is
// 0, the folder is assumed to be the root folder. Otherwise, the folder will be
// nested under the folder with that ID.
func (d *Database) InsertFolderForUser(u models.User, f models.Folder, parentId int64) (int64, error) {
	defer logElapsedTime(time.Now(), "InsertFolderForUser")

	var folderID int64

	err := d.db.QueryRow(
		`INSERT INTO Folder(userid, name) VALUES($1, $2)
    	 ON CONFLICT(name) DO UPDATE SET name = excluded.name RETURNING id`, u.UserId, f.Name).Scan(&folderID)
	if err != nil {
		return folderID, err
	}

	// TODO: Assert that there is not already a root folder.

	// If this is the root folder, then it has no parents, so no need to update
	// the folder mapping table.
	if parentId != 0 {
		_, err = d.db.Exec(
			`UPSERT INTO FolderChildren(userid, parent, child) VALUES($1, $2, $3)`, u.UserId, parentId, folderID)
		if err != nil {
			return folderID, err
		}
	}

	return folderID, nil
}

// DeleteArticlesForUser deletes all articles earlier than the given timestamp
// and returns the number deleted.
func (d *Database) DeleteArticlesForUser(u models.User, minTimestamp time.Time) (int64, error) {
	defer logElapsedTime(time.Now(), "DeleteArticlesForUser")

	r, err := d.db.Exec(
		`DELETE FROM Article WHERE userid = $1 AND read AND (retrieved IS NULL OR retrieved < $2) RETURNING id`,
		u.UserId, minTimestamp)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

// DeleteArticlesByIdForUser deletes articles in the given list of IDs for the given user.
func (d *Database) DeleteArticlesByIdForUser(u models.User, ids []int64) error {
	defer logElapsedTime(time.Now(), "DeleteArticlesByIdForUser")

	_, err := d.db.Exec(
		`DELETE FROM Article WHERE userid = $1 AND id = ANY($2)`, u.UserId, pq.Array(ids))
	return err
}

// DeleteFeedForUser deletes the specified feed and all articles under that feed.
func (d *Database) DeleteFeedForUser(u models.User, feedId int64, folderId int64) error {
	defer logElapsedTime(time.Now(), "DeleteFeedForUser")

	// TODO: Make the following queries into a single transaction.
	_, err := d.db.Exec(
		`DELETE FROM Article WHERE userid = $1 AND folder = $2 AND feed = $3`,
		u.UserId, folderId, feedId)
	if err != nil {
		return err
	}

	_, err = d.db.Exec(
		`DELETE FROM Feed WHERE userid = $1 AND folder = $2 AND id = $3`,
		u.UserId, folderId, feedId)

	return err
}

/*******************************************************************************
 * Modification methods
 ******************************************************************************/

// MarkArticleForUser sets the read status of the given article to the given
// status.
func (d *Database) MarkArticleForUser(u models.User, articleId int64, status string) error {
	defer logElapsedTime(time.Now(), "MarkArticleForUser")

	// TODO: Consider creating a type for the status string.
	state, err := parseState(status)
	if err != nil {
		return err
	}

	_, err = d.db.Exec(
		`UPDATE Article SET read = $1 WHERE userid = $2 AND id = $3`, state, u.UserId, articleId)
	return err
}

// MarkFeedForUser sets the read status of all articles in the given feed to the
// given status.
func (d *Database) MarkFeedForUser(u models.User, feedId int64, status string) error {
	defer logElapsedTime(time.Now(), "MarkFeedForUser")

	// TODO: Consider creating a type for the status string.
	state, err := parseState(status)
	if err != nil {
		return err
	}

	_, err = d.db.Exec(
		`UPDATE Article SET read = $1 WHERE userid = $2 AND feed = $3`, state, u.UserId, feedId)
	return err
}

// MarkFolderForUser sets the read status of all articles in the given folder to
// the given status. An ID of 0 will mark all articles in all folders to the
// given status.
func (d *Database) MarkFolderForUser(u models.User, folderId int64, status string) error {
	defer logElapsedTime(time.Now(), "MarkFolderForUser")

	// TODO: Consider creating a type for the status string.
	state, err := parseState(status)
	if err != nil {
		return err
	}

	// Special-case id=0 to mean everything (the root folder).
	if folderId == 0 {
		_, err = d.db.Exec(`UPDATE Article SET read = $1 WHERE userid = $2`, state, u.UserId)
		return err
	}

	_, err = d.db.Exec(
		`UPDATE Article SET read = $1 WHERE userid = $2 AND folder = $3`, state, u.UserId, folderId)
	if err != nil {
		return err
	}

	// Recursively mark child folders also with the same status.
	children, err := d.GetFolderChildrenForUser(u, folderId)
	if err != nil {
		return err
	}
	for _, c := range children {
		if err2 := d.MarkFolderForUser(u, c, status); err2 != nil {
			return err2
		}
	}
	return err
}

// UpdateFeedMetadataForUser updates various fields for the row corresponding to
// given models.Feed object with the values in that object.
func (d *Database) UpdateFeedMetadataForUser(u models.User, f models.Feed) error {
	defer logElapsedTime(time.Now(), "UpdateFeedMetadataForUser")

	_, err := d.db.Exec(
		`UPDATE Feed SET hash = $1, title = $2, description = $3, link = $4 WHERE userid = $5 AND folder = $6 AND id = $7`,
		f.Hash(), f.Title, f.Description, f.Link, u.UserId, f.FolderID, f.ID)
	return err
}

// UpdateLatestTimeForFeedForUser sets the latest retrieval time for the given
// feed to the given timestamp.
func (d *Database) UpdateLatestTimeForFeedForUser(u models.User, folderId int64, id int64, latest time.Time) error {
	defer logElapsedTime(time.Now(), "UpdateLatestTimeForFeedForUser")

	_, err := d.db.Exec(
		`UPDATE Feed SET latest = $1 WHERE userid = $2 AND folder = $3 AND id = $4`,
		latest, u.UserId, folderId, id)
	return err
}

/*******************************************************************************
 * Getter methods
 ******************************************************************************/

// GetFolderChildrenForUser returns a list of IDs corresponding to folders
// under the given folder ID.
func (d *Database) GetFolderChildrenForUser(u models.User, id int64) ([]int64, error) {
	defer logElapsedTime(time.Now(), "GetFolderChildrenForUser")

	var children []int64
	rows, err := d.db.Query(
		`SELECT child FROM FolderChildren WHERE userid = $1 AND parent = $2`, u.UserId, id)
	if err != nil {
		return children, err
	}
	defer closeSilent(rows)

	var childID int64
	for rows.Next() {
		if err = rows.Scan(&childID); err != nil {
			return children, err
		}
		children = append(children, childID)
	}
	return children, err
}

// GetAllFoldersForUser returns a list of all folders in the database for the
// given user.
func (d *Database) GetAllFoldersForUser(u models.User) ([]models.Folder, error) {
	defer logElapsedTime(time.Now(), "GetAllFoldersForUser")

	// TODO: Consider returning a map[int64]models.Folder instead.
	var folders []models.Folder
	rows, err := d.db.Query(`SELECT id, name FROM Folder WHERE userid = $1`, u.UserId)
	if err != nil {
		return folders, err
	}
	defer closeSilent(rows)

	for rows.Next() {
		f := models.Folder{}
		if err = rows.Scan(&f.ID, &f.Name); err != nil {
			return folders, err
		}
		folders = append(folders, f)
	}

	return folders, err
}

// GetAllFeedsForUser returns a list of all feeds in the database for the
// given user.
func (d *Database) GetAllFeedsForUser(u models.User) ([]models.Feed, error) {
	defer logElapsedTime(time.Now(), "GetAllFeedsForUser")

	var feeds []models.Feed
	rows, err := d.db.Query(
		`SELECT id, folder, title, description, url, link, latest FROM Feed WHERE userid = $1`, u.UserId)
	if err != nil {
		return feeds, err
	}
	defer closeSilent(rows)

	for rows.Next() {
		f := models.Feed{}
		if err = rows.Scan(&f.ID, &f.FolderID, &f.Title, &f.Description, &f.URL, &f.Link, &f.Latest); err != nil {
			return feeds, err
		}
		feeds = append(feeds, f)
	}

	return feeds, err
}

// GetFeedsInFolderForUser returns a list of feeds directly under the given
// folder for the given user.
func (d *Database) GetFeedsInFolderForUser(u models.User, folderId int64) ([]models.Feed, error) {
	defer logElapsedTime(time.Now(), "GetFeedsInFolderForUser")

	var feeds []models.Feed

	rows, err := d.db.Query(
		`SELECT id, title, url FROM Feed WHERE userid = $1 AND folder = $2`, u.UserId, folderId)
	if err != nil {
		return feeds, err
	}
	defer closeSilent(rows)

	for rows.Next() {
		feed := models.Feed{}
		if err := rows.Scan(&feed.ID, &feed.Title, &feed.URL); err != nil {
			return feeds, err
		}
		feeds = append(feeds, feed)
	}
	return feeds, nil
}

// GetFeedsPerFolderForUser returns a map of folder ID to an array of feed IDs.
func (d *Database) GetFeedsPerFolderForUser(u models.User) (map[int64][]int64, error) {
	defer logElapsedTime(time.Now(), "GetFeedsPerFolderForUser")

	resp := map[int64][]int64{}

	rows, err := d.db.Query(`SELECT folder, id FROM Feed WHERE userid = $1`, u.UserId)
	if err != nil {
		return resp, err
	}
	defer closeSilent(rows)

	var folderID, feedID int64
	for rows.Next() {
		if err = rows.Scan(&folderID, &feedID); err != nil {
			return resp, err
		}
		resp[folderID] = append(resp[folderID], feedID)
	}

	return resp, err
}

// GetFolderFeedTreeForUser returns a root Folder object with associated feeds
// and recursively populated sub-folders.
func (d *Database) GetFolderFeedTreeForUser(u models.User) (*models.Folder, error) {
	defer logElapsedTime(time.Now(), "GetFolderFeedTreeForUser")

	rootFolder := &models.Folder{}
	var rootId int64

	// First determine the root ID
	err := d.db.QueryRow(
		`SELECT id from Folder WHERE userid = $1 AND name = $2`, u.UserId, models.RootFolder).Scan(&rootId)
	if err != nil {
		return rootFolder, err
	}
	rootFolder.ID = rootId
	rootFolder.Name = "Root"

	// Create map from ID to Folder
	folders, err := d.GetAllFoldersForUser(u)
	if err != nil {
		return rootFolder, err
	}
	folderMap := map[int64]models.Folder{}
	for _, f := range folders {
		folderMap[f.ID] = f
	}

	// Forward declaration so that this can be recursively invoked.
	var buildTree func(*models.Folder) error

	// Given a pointer to a folder, populate all feeds and folders within it.
	buildTree = func(node *models.Folder) error {
		feeds, err := d.GetFeedsInFolderForUser(u, node.ID)
		if err != nil {
			return err
		}
		node.Feed = feeds

		childFolders, err := d.GetFolderChildrenForUser(u, node.ID)
		if err != nil {
			return err
		}

		for _, c := range childFolders {
			ch := folderMap[c]
			child := &models.Folder{ID: ch.ID, Name: ch.Name}
			err := buildTree(child)
			if err != nil {
				return err
			}
			node.Folders = append(node.Folders, *child)
		}
		return nil
	}

	err = buildTree(rootFolder)
	return rootFolder, err
}

// GetAllFaviconsForUser returns a map of feed ID to a base64 representation of
// its favicon. Feeds with no favicons are not part of the returned map.
func (d *Database) GetAllFaviconsForUser(u models.User) (map[int64]string, error) {
	defer logElapsedTime(time.Now(), "GetAllFaviconsForUser")

	// TODO: Consider returning a Favicon model type.
	favicons := map[int64]string{}
	rows, err := d.db.Query(
		`SELECT id, mime, favicon FROM Feed WHERE userid = $1 AND favicon IS NOT NULL`, u.UserId)
	if err != nil {
		return favicons, err
	}
	defer closeSilent(rows)

	var id int64
	var mime string
	var favicon string
	for rows.Next() {
		if err = rows.Scan(&id, &mime, &favicon); err != nil {
			return favicons, err
		}
		favicons[id] = fmt.Sprintf("%s;base64,%s", mime, favicon)
	}
	return favicons, err
}

// GetUnreadArticleMetaForUser returns a list of at most the given limit of
// articles after the given ID. Only metadata fields are returned, not content.
func (d *Database) GetUnreadArticleMetaForUser(u models.User, limit int, sinceID int64) ([]models.Article, error) {
	defer logElapsedTime(time.Now(), "GetUnreadArticlesForUser")

	var articles []models.Article
	var rows *sql.Rows
	var err error

	if limit == -1 {
		limit = maxFetchedRows
	}
	if sinceID == -1 {
		sinceID = 0
	}

	rows, err = d.db.Query(
		`SELECT id, feed, folder, date FROM Article
		WHERE userid = $1 AND id > $2 AND NOT read ORDER BY id LIMIT $3`, u.UserId, sinceID, limit)
	if err != nil {
		return articles, err
	}
	defer closeSilent(rows)

	for rows.Next() {
		a := models.Article{}
		if err = rows.Scan(
			&a.ID, &a.FeedID, &a.FolderID, &a.Date); err != nil {
			return articles, err
		}
		articles = append(articles, a)
	}
	return articles, err
}

// GetArticlesForUser returns articles from the specified list.
func (d *Database) GetArticlesForUser(u models.User, ids []int64) ([]models.Article, error) {
	defer logElapsedTime(time.Now(), "GetArticlesForUser")

	var articles []models.Article
	var rows *sql.Rows
	var err error

	rows, err = d.db.Query(
		`SELECT id, feed, folder, title, summary, content, parsed, link, date FROM Article
		WHERE userid = $1 AND id = ANY($2)`, u.UserId, pq.Array(ids))
	if err != nil {
		return articles, err
	}
	defer closeSilent(rows)

	for rows.Next() {
		a := models.Article{}
		if err = rows.Scan(
			&a.ID, &a.FeedID, &a.FolderID, &a.Title, &a.Summary, &a.Content, &a.Parsed, &a.Link, &a.Date); err != nil {
			return articles, err
		}
		articles = append(articles, a)
	}
	return articles, err
}

// GetUnreadArticlesForUser returns a list of at most the given limit of
// articles after the given ID.
func (d *Database) GetUnreadArticlesForUser(u models.User, limit int, sinceID int64) ([]models.Article, error) {
	defer logElapsedTime(time.Now(), "GetUnreadArticlesForUser")

	var articles []models.Article
	var rows *sql.Rows
	var err error

	if limit == -1 {
		limit = maxFetchedRows
	}
	if sinceID == -1 {
		sinceID = 0
	}

	rows, err = d.db.Query(
		`SELECT id, feed, folder, title, summary, content, parsed, link, date FROM Article
		WHERE userid = $1 AND id > $2 AND NOT read ORDER BY id LIMIT $3`, u.UserId, sinceID, limit)
	if err != nil {
		return articles, err
	}
	defer closeSilent(rows)

	for rows.Next() {
		a := models.Article{}
		if err = rows.Scan(
			&a.ID, &a.FeedID, &a.FolderID, &a.Title, &a.Summary, &a.Content, &a.Parsed, &a.Link, &a.Date); err != nil {
			return articles, err
		}
		articles = append(articles, a)
	}
	return articles, err
}

// GetArticlesForFeedForUser returns a list of articles for the
// given feed ID and user.
func (d *Database) GetArticlesForFeedForUser(u models.User, feedId int64) ([]models.Article, error) {
	defer logElapsedTime(time.Now(), "GetUnreadArticlesForFeedForUser")

	var articles []models.Article
	var rows *sql.Rows
	var err error

	rows, err = d.db.Query(
		`SELECT id, feed, folder, title, summary, content, parsed, link, read, date FROM Article
		WHERE userid = $1 AND feed = $2`, u.UserId, feedId)
	if err != nil {
		return articles, err
	}
	defer closeSilent(rows)

	for rows.Next() {
		a := models.Article{}
		if err = rows.Scan(
			&a.ID, &a.FeedID, &a.FolderID, &a.Title, &a.Summary, &a.Content, &a.Parsed, &a.Link, &a.Read, &a.Date); err != nil {
			return articles, err
		}
		articles = append(articles, a)
	}
	return articles, err
}

/*******************************************************************************
 * Import methods
 ******************************************************************************/

// ImportOpmlForUser inserts folders from the given OPML object into the
// database for the given user.
func (d *Database) ImportOpmlForUser(u models.User, opml *opml.Opml) error {
	root := opml.Folders
	rootID, err := d.InsertFolderForUser(u, root, 0)
	if err != nil {
		return err
	}
	root.ID = rootID

	return d.importChildrenForUser(u, root)
}

func (d *Database) importChildrenForUser(u models.User, parent models.Folder) error {
	var err error
	for _, f := range parent.Feed {
		feedID, err := d.InsertFeedForUser(u, f, parent.ID)
		if err != nil {
			return err
		}
		f.ID = feedID
	}

	for _, child := range parent.Folders {
		childID, err := d.InsertFolderForUser(u, child, parent.ID)
		if err != nil {
			return err
		}
		child.ID = childID

		if err = d.importChildrenForUser(u, child); err != nil {
			return err
		}
	}
	return err
}

/*******************************************************************************
 * Helper methods
 ******************************************************************************/

func parseState(status string) (bool, error) {
	var state bool
	switch status {
	case "saved", "unsaved":
		// Perhaps add support for saving articles in the future.
		return false, errors.New("Unsupported status: %s" + status)
	case "read":
		state = true
	case "unread":
		state = false
	}
	return state, nil
}

func logElapsedTime(t time.Time, method string) {
	utils.Elapsed(t, func(d time.Duration) {
		// Record latency measurements in microseconds.
		latencyMetric.WithLabelValues(method).Observe(float64(d) / float64(time.Microsecond))
		if d > slowOpLogThreshold {
			log.V(2).Infof("Slow operation for method %s: %s", method, d)
		}
	})
}

func closeSilent(rows *sql.Rows) {
	err := rows.Close()
	if err != nil {
		log.Warningf("Failed to close rows: %+v", rows)
	}
}
