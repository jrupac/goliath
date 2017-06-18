package storage

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	// PostgreSQL driver support
	_ "github.com/lib/pq"
	"strconv"
	"strings"
	"time"
)

const (
	DIALECT = "postgres"
	FOLDER_TABLE = "Folder"
	FOLDER_CHILDREN_TABLE = "FolderChildren"
	FEED_TABLE = "Feed"
	ARTICLE_TABLE = "Article"
	USER_TABLE = "UserTable"
	MAX_FETCHED_ROWS = 10000
)

type Database struct {
	db *sql.DB
}

func Open(dbPath string) (*Database, error) {
	db := new(Database)

	d, err := sql.Open(DIALECT, dbPath)
	if err != nil {
		return nil, err
	}
	if err = d.Ping(); err != nil {
		return nil, err
	}

	db.db = d
	return db, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

/*******************************************************************************
 * Insertion/deletion methods
 ******************************************************************************/

func (d *Database) InsertArticle(a models.Article) error {
	var articleId int64
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM ` + ARTICLE_TABLE + ` WHERE hash = $1`, a.Hash()).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		log.V(2).Infof("Duplicate article entry, skipping (hash): %s", a.Hash())
		return nil
	}

	err = d.db.QueryRow(
		`INSERT INTO ` + ARTICLE_TABLE + `
		(feed, folder, hash, title, summary, content, link, read, date, retrieved)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
		a.FeedId, a.FolderId, a.Hash(), a.Title, a.Summary, a.Content, a.Link, a.Read, a.Date, a.Retrieved).Scan(&articleId)
	if err != nil {
		return err
	}
	a.Id = articleId
	return nil
}

func (d *Database) InsertFavicon(feedId int64, mime string, img []byte) error {
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM ` + FEED_TABLE + ` WHERE id = $1`, feedId).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("Original feed not in table: %d", feedId)
	}

	// Convert to a base64 encoded string before inserting
	h := base64.StdEncoding.EncodeToString(img)

	_, err = d.db.Exec(
		`UPDATE ` + FEED_TABLE + ` SET favicon = $1, mime = $2 WHERE id = $3`,
		h, mime, feedId)
	return err
}

func (d *Database) InsertUser(u models.User) error {
	_, err := d.db.Exec(`INSERT INTO ` + USER_TABLE + `(username, key) VALUES($1, $2)`, u.Username, u.Key)
	return err
}

func (d *Database) DeleteArticles(minTimestamp time.Time) (int64, error) {
	r, err := d.db.Exec(
		`DELETE FROM ` + ARTICLE_TABLE + ` WHERE read AND (retrieved IS NULL OR retrieved < $1) RETURNING id`, minTimestamp)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

/*******************************************************************************
 * Modification methods
 ******************************************************************************/

func (d *Database) MarkArticle(id int64, status string) error {
	state, err := parseState(status)
	if err != nil {
		return err
	}

	_, err = d.db.Exec(`UPDATE ` + ARTICLE_TABLE + ` SET read = $1 WHERE id = $2`, state, id)
	return err
}

func (d *Database) MarkFeed(id int64, status string) error {
	state, err := parseState(status)
	if err != nil {
		return err
	}

	_, err = d.db.Exec(`UPDATE ` + ARTICLE_TABLE + ` SET read = $1 WHERE feed = $2`, state, id)
	return err
}

func (d *Database) MarkFolder(id int64, status string) error {
	state, err := parseState(status)
	if err != nil {
		return err
	}

	_, err = d.db.Exec(`UPDATE ` + ARTICLE_TABLE + ` SET read = $1 WHERE folder = $2`, state, id)
	children, err := d.GetFolderChildren(id)
	if err != nil {
		return err
	}
	for _, c := range children {
		if err := d.MarkFolder(c, status); err != nil {
			return err
		}
	}
	return err
}

func (d *Database) UpdateLatestTimeForFeed(id int64, latest time.Time) error {
	_, err := d.db.Exec(`UPDATE ` + FEED_TABLE + ` SET latest = $1 WHERE id = $2`, latest, id)
	return err
}

/*******************************************************************************
 * Getter methods
 ******************************************************************************/

func (d *Database) GetFolderChildren(id int64) ([]int64, error) {
	children := []int64{}
	rows, err := d.db.Query(`SELECT child FROM ` + FOLDER_CHILDREN_TABLE + ` WHERE parent = $1`, id)
	defer rows.Close()
	if err != nil {
		return children, err
	}

	var childId int64
	for rows.Next() {
		if err = rows.Scan(&childId); err != nil {
			return children, err
		}
		children = append(children, childId)
	}
	return children, err
}

func (d *Database) GetAllFolders() ([]models.Folder, error) {
	folders := []models.Folder{}
	rows, err := d.db.Query(`SELECT id, name FROM ` + FOLDER_TABLE)
	defer rows.Close()
	if err != nil {
		return folders, err
	}

	for rows.Next() {
		f := models.Folder{}
		if err = rows.Scan(&f.Id, &f.Name); err != nil {
			return folders, err
		}
		folders = append(folders, f)
	}

	return folders, err
}

func (d *Database) GetAllFeeds() ([]models.Feed, error) {
	feeds := []models.Feed{}
	rows, err := d.db.Query(`SELECT id, folder, title, description, url, latest FROM ` + FEED_TABLE)
	defer rows.Close()
	if err != nil {
		return feeds, err
	}

	for rows.Next() {
		f := models.Feed{}
		if err = rows.Scan(&f.Id, &f.FolderId, &f.Title, &f.Description, &f.Url, &f.Latest); err != nil {
			return feeds, err
		}
		feeds = append(feeds, f)
	}

	return feeds, err
}

func (d *Database) GetFeedsPerFolder() (map[int64]string, error) {
	// TODO: Make this method return map[int64][]int64.
	// Right now this is encoding Fever API semantics into the DB function.
	agg := map[int64][]string{}
	resp := map[int64]string{}

	// CockroachDB doesn't have a concat-with-separator aggregation function
	rows, err := d.db.Query(`SELECT folder, id FROM ` + FEED_TABLE)
	defer rows.Close()
	if err != nil {
		return resp, err
	}

	var folderId, feedId int64
	for rows.Next() {
		if err = rows.Scan(&folderId, &feedId); err != nil {
			return resp, err
		}
		agg[folderId] = append(agg[folderId], strconv.FormatInt(feedId, 10))
	}

	for k, v := range agg {
		resp[k] = strings.Join(v, ",")
	}
	return resp, err
}

func (d *Database) GetAllFavicons() (map[int64]string, error) {
	favicons := map[int64]string{}
	rows, err := d.db.Query(
		`SELECT id, mime, favicon FROM ` + FEED_TABLE + ` WHERE favicon IS NOT NULL`)
	defer rows.Close()
	if err != nil {
		return favicons, err
	}

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

func (d *Database) GetUnreadArticles(limit int, sinceId int64) ([]models.Article, error) {
	articles := []models.Article{}
	var rows *sql.Rows
	var err error

	if limit == -1 {
		limit = MAX_FETCHED_ROWS
	}
	if sinceId == -1 {
		sinceId = 0
	}

	rows, err = d.db.Query(
		`SELECT id, feed, folder, title, summary, content, link, date FROM ` + ARTICLE_TABLE + `
		WHERE NOT read AND id > $1 ORDER BY id LIMIT $2`, sinceId, limit)
	defer rows.Close()
	if err != nil {
		return articles, err
	}

	for rows.Next() {
		a := models.Article{}
		if err = rows.Scan(
			&a.Id, &a.FeedId, &a.FolderId, &a.Title, &a.Summary, &a.Content, &a.Link, &a.Date); err != nil {
			return articles, err
		}
		articles = append(articles, a)
	}
	return articles, err
}

func (d *Database) GetUserByKey(key string) (models.User, error) {
	var u models.User
	err := d.db.QueryRow(
		`SELECT username, key FROM ` + USER_TABLE + ` WHERE key = $1`, key).Scan(
		&u.Username, &u.Key)
	if !u.Valid() {
		return models.User{}, errors.New("Could not find user.")
	}
	return u, err
}

/*******************************************************************************
 * Import methods
 ******************************************************************************/

func (d *Database) ImportOpml(opml *models.Opml) error {
	root := opml.Folders
	// TODO: Remove extra read after https://github.com/cockroachdb/cockroach/issues/6637 is closed.
	_, err := d.db.Exec(
		`INSERT INTO ` + FOLDER_TABLE + `(name) VALUES($1) ON CONFLICT(name) DO NOTHING`, root.Name)
	if err != nil {
		return err
	}
	err = d.db.QueryRow(
		`SELECT id FROM ` + FOLDER_TABLE + ` WHERE name = $1`, root.Name).Scan(&root.Id)
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
			`INSERT INTO ` + FEED_TABLE + `(folder, hash, title, description, url)
			VALUES($1, $2, $3, $4, $5) ON CONFLICT(hash) DO NOTHING`,
			parent.Id, f.Hash(), f.Title, f.Description, f.Url)
		if err != nil {
			return err
		}
		err = d.db.QueryRow(
			`SELECT id FROM ` + FEED_TABLE + ` WHERE hash = $1`, f.Hash()).Scan(&f.Id)
		if err != nil {
			return err
		}
		f.FolderId = parent.Id
	}

	for _, child := range parent.Folders {
		// TODO: Remove extra read after https://github.com/cockroachdb/cockroach/issues/6637 is closed.
		_, err := d.db.Exec(
			`INSERT INTO ` + FOLDER_TABLE + `(name) VALUES($1) ON CONFLICT(name) DO NOTHING`, child.Name)
		if err != nil {
			return err
		}
		err = d.db.QueryRow(
			`SELECT id FROM ` + FOLDER_TABLE + ` WHERE name = $1`, child.Name).Scan(&child.Id)
		if err != nil {
			return err
		}

		_, err = d.db.Exec(
			`UPSERT INTO ` + FOLDER_CHILDREN_TABLE + `(parent, child) VALUES($1, $2)`, parent.Id, child.Id)
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
