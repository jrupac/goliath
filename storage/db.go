package storage

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/opml"

	// PostgreSQL driver support
	_ "github.com/lib/pq"
	"strconv"
	"strings"
	"time"
)

const (
	dialect             = "postgres"
	folderTable         = "Folder"
	folderChildrenTable = "FolderChildren"
	feedTable           = "Feed"
	articleTable        = "Article"
	userTable           = "UserTable"
	maxFetchedRows      = 10000
)

// Database is a wrapper type around a database connection.
type Database struct {
	db *sql.DB
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
 * Insertion/deletion methods
 ******************************************************************************/

// InsertArticle inserts the given article object into the database.
func (d *Database) InsertArticle(a models.Article) error {
	var articleID int64
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM `+articleTable+` WHERE hash = $1`, a.Hash()).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		log.V(2).Infof("Duplicate article entry, skipping (hash): %s", a.Hash())
		return nil
	}

	err = d.db.QueryRow(
		`INSERT INTO `+articleTable+`
		(feed, folder, hash, title, summary, content, parsed, link, read, date, retrieved)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
		a.FeedID, a.FolderID, a.Hash(), a.Title, a.Summary, a.Content, a.Parsed, a.Link, a.Read, a.Date, a.Retrieved).Scan(&articleID)
	if err != nil {
		return err
	}
	a.ID = articleID
	return nil
}

// InsertFavicon inserts the given favicon and associated metadata into the database.
func (d *Database) InsertFavicon(feedID int64, mime string, img []byte) error {
	// TODO: Consider wrapping this into a Favicon model type.
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM `+feedTable+` WHERE id = $1`, feedID).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("original feed not in table: %d", feedID)
	}

	// Convert to a base64 encoded string before inserting
	h := base64.StdEncoding.EncodeToString(img)

	_, err = d.db.Exec(
		`UPDATE `+feedTable+` SET favicon = $1, mime = $2 WHERE id = $3`,
		h, mime, feedID)
	return err
}

// InsertUser inserts the given user into the database.
func (d *Database) InsertUser(u models.User) error {
	_, err := d.db.Exec(`INSERT INTO `+userTable+`(username, key) VALUES($1, $2)`, u.Username, u.Key)
	return err
}

// DeleteArticles deletes all articles earlier than the given timestamp and returns the number deleted.
func (d *Database) DeleteArticles(minTimestamp time.Time) (int64, error) {
	r, err := d.db.Exec(
		`DELETE FROM `+articleTable+` WHERE read AND (retrieved IS NULL OR retrieved < $1) RETURNING id`, minTimestamp)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

/*******************************************************************************
 * Modification methods
 ******************************************************************************/

// MarkArticle sets the read status of the given article to the given status.
func (d *Database) MarkArticle(id int64, status string) error {
	// TODO: Consider creating a type for the status string.
	state, err := parseState(status)
	if err != nil {
		return err
	}

	_, err = d.db.Exec(`UPDATE `+articleTable+` SET read = $1 WHERE id = $2`, state, id)
	return err
}

// MarkFeed sets the read status of all articles in the given feed to the given status.
func (d *Database) MarkFeed(id int64, status string) error {
	// TODO: Consider creating a type for the status string.
	state, err := parseState(status)
	if err != nil {
		return err
	}

	_, err = d.db.Exec(`UPDATE `+articleTable+` SET read = $1 WHERE feed = $2`, state, id)
	return err
}

// MarkFolder sets the read status of all articles in the given folder to the given status.
// An ID of 0 will mark all articles in all folders to the given status.
func (d *Database) MarkFolder(id int64, status string) error {
	// TODO: Consider creating a type for the status string.
	state, err := parseState(status)
	if err != nil {
		return err
	}

	// Special-case id=0 to mean everything (the root folder).
	if id == 0 {
		_, err = d.db.Exec(`UPDATE `+articleTable+` SET read = $1`, state)
		return err
	}

	_, err = d.db.Exec(`UPDATE `+articleTable+` SET read = $1 WHERE folder = $2`, state, id)
	if err != nil {
		return err
	}
	children, err := d.GetFolderChildren(id)
	if err != nil {
		return err
	}
	for _, c := range children {
		if err2 := d.MarkFolder(c, status); err2 != nil {
			return err2
		}
	}
	return err
}

// UpdateLatestTimeForFeed sets the latest retrieval time for the given feed to the given timestamp.
func (d *Database) UpdateLatestTimeForFeed(id int64, latest time.Time) error {
	_, err := d.db.Exec(`UPDATE `+feedTable+` SET latest = $1 WHERE id = $2`, latest, id)
	return err
}

/*******************************************************************************
 * Getter methods
 ******************************************************************************/

// GetFolderChildren returns a list of IDs corresponding to folders under the given folder ID.
func (d *Database) GetFolderChildren(id int64) ([]int64, error) {
	var children []int64
	rows, err := d.db.Query(`SELECT child FROM `+folderChildrenTable+` WHERE parent = $1`, id)
	if err != nil {
		return children, err
	}
	defer rows.Close()

	var childID int64
	for rows.Next() {
		if err = rows.Scan(&childID); err != nil {
			return children, err
		}
		children = append(children, childID)
	}
	return children, err
}

// GetAllFolders returns a list of all folders in the database.
func (d *Database) GetAllFolders() ([]models.Folder, error) {
	var folders []models.Folder
	rows, err := d.db.Query(`SELECT id, name FROM ` + folderTable)
	if err != nil {
		return folders, err
	}
	defer rows.Close()

	for rows.Next() {
		f := models.Folder{}
		if err = rows.Scan(&f.ID, &f.Name); err != nil {
			return folders, err
		}
		folders = append(folders, f)
	}

	return folders, err
}

// GetAllFeeds returns a list of all feeds in the database.
func (d *Database) GetAllFeeds() ([]models.Feed, error) {
	var feeds []models.Feed
	rows, err := d.db.Query(`SELECT id, folder, title, description, url, latest FROM ` + feedTable)
	if err != nil {
		return feeds, err
	}
	defer rows.Close()

	for rows.Next() {
		f := models.Feed{}
		if err = rows.Scan(&f.ID, &f.FolderID, &f.Title, &f.Description, &f.URL, &f.Latest); err != nil {
			return feeds, err
		}
		feeds = append(feeds, f)
	}

	return feeds, err
}

// GetFeedsPerFolder returns a map of folder ID to a comma-separated string of feed IDs.
func (d *Database) GetFeedsPerFolder() (map[int64]string, error) {
	// TODO: Make this method return map[int64][]int64.
	// Right now this is encoding Fever API semantics into the DB function.
	agg := map[int64][]string{}
	resp := map[int64]string{}

	// CockroachDB doesn't have a concat-with-separator aggregation function
	rows, err := d.db.Query(`SELECT folder, id FROM ` + feedTable)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	var folderID, feedID int64
	for rows.Next() {
		if err = rows.Scan(&folderID, &feedID); err != nil {
			return resp, err
		}
		agg[folderID] = append(agg[folderID], strconv.FormatInt(feedID, 10))
	}

	for k, v := range agg {
		resp[k] = strings.Join(v, ",")
	}
	return resp, err
}

// GetAllFavicons returns a map of feed ID to a base64 representation of its favicon.
// Feeds with no favicons are not part of the returned map.
func (d *Database) GetAllFavicons() (map[int64]string, error) {
	// TODO: Consider returning a Favicon model type.
	favicons := map[int64]string{}
	rows, err := d.db.Query(
		`SELECT id, mime, favicon FROM ` + feedTable + ` WHERE favicon IS NOT NULL`)
	if err != nil {
		return favicons, err
	}
	defer rows.Close()

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

// GetUnreadArticles returns a list of at most the given limit of articles after the given ID.
func (d *Database) GetUnreadArticles(limit int, sinceID int64) ([]models.Article, error) {
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
		`SELECT id, feed, folder, title, summary, content, parsed, link, date FROM `+articleTable+`
		WHERE NOT read AND id > $1 ORDER BY id LIMIT $2`, sinceID, limit)
	if err != nil {
		return articles, err
	}
	defer rows.Close()

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

// GetUserByKey returns a user identified by the given key.
func (d *Database) GetUserByKey(key string) (models.User, error) {
	var u models.User
	err := d.db.QueryRow(
		`SELECT username, key FROM `+userTable+` WHERE key = $1`, key).Scan(
		&u.Username, &u.Key)
	if !u.Valid() {
		return models.User{}, errors.New("could not find user")
	}
	return u, err
}

/*******************************************************************************
 * Import methods
 ******************************************************************************/

// ImportOpml inserts folders from the given OPML object into the database.
func (d *Database) ImportOpml(opml *opml.Opml) error {
	root := opml.Folders
	// TODO: Remove extra read after https://github.com/cockroachdb/cockroach/issues/6637 is closed.
	_, err := d.db.Exec(
		`INSERT INTO `+folderTable+`(name) VALUES($1) ON CONFLICT(name) DO NOTHING`, root.Name)
	if err != nil {
		return err
	}
	err = d.db.QueryRow(
		`SELECT id FROM `+folderTable+` WHERE name = $1`, root.Name).Scan(&root.ID)
	if err != nil {
		return err
	}
	return d.importChildren(root)
}

func (d *Database) importChildren(parent models.Folder) error {
	var err error
	for _, f := range parent.Feed {
		// TODO: Remove extra read after https://github.com/cockroachdb/cockroach/issues/6637 is closed.
		_, err = d.db.Exec(
			`INSERT INTO `+feedTable+`(folder, hash, title, description, url)
			VALUES($1, $2, $3, $4, $5) ON CONFLICT(hash) DO NOTHING`,
			parent.ID, f.Hash(), f.Title, f.Description, f.URL)
		if err != nil {
			return err
		}
		err = d.db.QueryRow(
			`SELECT id FROM `+feedTable+` WHERE hash = $1`, f.Hash()).Scan(&f.ID)
		if err != nil {
			return err
		}
		f.FolderID = parent.ID
	}

	for _, child := range parent.Folders {
		// TODO: Remove extra read after https://github.com/cockroachdb/cockroach/issues/6637 is closed.
		_, err = d.db.Exec(
			`INSERT INTO `+folderTable+`(name) VALUES($1) ON CONFLICT(name) DO NOTHING`, child.Name)
		if err != nil {
			return err
		}
		err = d.db.QueryRow(
			`SELECT id FROM `+folderTable+` WHERE name = $1`, child.Name).Scan(&child.ID)
		if err != nil {
			return err
		}

		_, err = d.db.Exec(
			`UPSERT INTO `+folderChildrenTable+`(parent, child) VALUES($1, $2)`, parent.ID, child.ID)
		if err != nil {
			return err
		}
		if err = d.importChildren(child); err != nil {
			return err
		}
	}
	return err
}

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
