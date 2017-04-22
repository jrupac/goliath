package storage

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	_ "github.com/lib/pq"
	"strconv"
	"strings"
)

const (
	DIALECT               = "postgres"
	FOLDER_TABLE          = "Folder"
	FOLDER_CHILDREN_TABLE = "FolderChildren"
	FEED_TABLE            = "Feed"
	ARTICLE_TABLE         = "Article"
	MAX_FETCHED_ROWS      = 10000
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

	db.db = d
	return db, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) InsertArticle(a models.Article) error {
	var articleId int64
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM `+ARTICLE_TABLE+` WHERE hash = $1`, a.Hash()).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		log.Infof("Duplicate article entry, skipping.")
		return nil
	}

	err = d.db.QueryRow(
		`INSERT INTO `+ARTICLE_TABLE+`
		(feed, folder, hash, title, summary, content, link, date, read)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		a.FeedId, a.FolderId, a.Hash(), a.Title, a.Summary, a.Content, a.Link, a.Date, a.Read).Scan(&articleId)
	if err != nil {
		return err
	}
	a.Id = articleId
	return nil
}

func (d *Database) InsertFavicon(feedId int64, mime string, img []byte) error {
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM `+FEED_TABLE+` WHERE id = $1`, feedId).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New(fmt.Sprintf("Original feed not in table: %d", feedId))
	}

	// Convert to a base64 encoded string before inserting
	h := base64.StdEncoding.EncodeToString(img)

	_, err = d.db.Query(
		`UPDATE `+FEED_TABLE+` SET favicon = $1, mime = $2 WHERE id = $3`,
		h, mime, feedId)
	return err
}

func (d *Database) MarkArticle(id int64, status string) error {
	var state bool
	switch status {
	case "saved", "unsaved":
		// Perhaps add support for saving articles in the future.
		return nil
	case "read":
		state = true
	case "unread":
		state = false
	}

	_, err := d.db.Query(`UPDATE `+ARTICLE_TABLE+` SET read = $1 WHERE id = $2`, state, id)
	return err
}

func (d *Database) MarkFeed(id int64, status string) error {
	var state bool
	switch status {
	case "saved", "unsaved":
		// Perhaps add support for saving articles in the future.
		return nil
	case "read":
		state = true
	case "unread":
		state = false
	}

	_, err := d.db.Query(`UPDATE `+ARTICLE_TABLE+` SET read = $1 WHERE feed = $2`, state, id)
	return err
}

func (d *Database) MarkFolder(id int64, status string) error {
	var state bool
	switch status {
	case "saved", "unsaved":
		// Perhaps add support for saving articles in the future.
		return nil
	case "read":
		state = true
	case "unread":
		state = false
	}

	_, err := d.db.Query(`UPDATE `+ARTICLE_TABLE+` SET read = $1 WHERE folder = $2`, state, id)
	return err
}

func (d *Database) GetAllFolders() ([]models.Folder, error) {
	folders := []models.Folder{}
	rows, err := d.db.Query(`SELECT id, name FROM ` + FOLDER_TABLE)
	if err != nil {
		return folders, err
	}
	defer rows.Close()

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
	rows, err := d.db.Query(`SELECT id, folder, title, description, url FROM ` + FEED_TABLE)
	if err != nil {
		return feeds, err
	}
	defer rows.Close()

	for rows.Next() {
		f := models.Feed{}
		if err = rows.Scan(&f.Id, &f.FolderId, &f.Title, &f.Description, &f.Url); err != nil {
			return feeds, err
		}
		feeds = append(feeds, f)
	}

	return feeds, err
}

func (d *Database) GetFeedsPerFolder() (map[int64]string, error) {
	agg := map[int64][]string{}
	resp := map[int64]string{}

	// CockroachDB doesn't have a concat-with-separator aggregation function
	rows, err := d.db.Query(`SELECT folder, id FROM ` + FEED_TABLE)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	var folderId, feed_id int64
	for rows.Next() {
		if err = rows.Scan(&folderId, &feed_id); err != nil {
			return resp, err
		}
		agg[folderId] = append(agg[folderId], strconv.FormatInt(feed_id, 10))
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

func (d *Database) GetUnreadArticles(limit int, since_id int64) ([]models.Article, error) {
	articles := []models.Article{}
	var rows *sql.Rows
	var err error

	if limit == -1 {
		limit = MAX_FETCHED_ROWS
	}
	if since_id == -1 {
		since_id = 0
	}

	rows, err = d.db.Query(
		`SELECT id, feed, folder, title, summary, content, link, date FROM `+ARTICLE_TABLE+`
		WHERE NOT read AND id > $1 LIMIT $2`, since_id, limit)

	if err != nil {
		return articles, err
	}
	defer rows.Close()

	var dateStr string
	for rows.Next() {
		a := models.Article{}
		if err = rows.Scan(
			&a.Id, &a.FeedId, &a.FolderId, &a.Title, &a.Summary, &a.Content, &a.Link, &dateStr); err != nil {
			return articles, err
		}
		articles = append(articles, a)
	}
	return articles, err
}

func (d *Database) ImportOpml(opml *models.Opml) error {
	root := opml.Folders
	var rootId int64
	err := d.db.QueryRow(
		`INSERT INTO `+FOLDER_TABLE+`(name) VALUES($1) RETURNING id`, root.Name).Scan(&rootId)
	if err != nil {
		return err
	}
	root.Id = rootId
	return d.importChildren(root)
}

func (d *Database) importChildren(parent models.Folder) error {
	var err error
	var feedId int64
	var count int
	for _, f := range parent.Feed {
		err := d.db.QueryRow(
			`SELECT COUNT(*) FROM `+FEED_TABLE+` WHERE hash = $1`, f.Hash()).Scan(&count)
		if err != nil {
			return err
		}
		if count > 0 {
			log.Infof("Duplicate feed entry, skipping.")
			continue
		}

		err = d.db.QueryRow(
			`INSERT INTO `+FEED_TABLE+`(folder, hash, title, description, url)
			VALUES($1, $2, $3, $4, $5) RETURNING id`,
			parent.Id, f.Hash(), f.Title, f.Description, f.Url).Scan(&feedId)
		if err != nil {
			return err
		}
		f.Id = feedId
		f.FolderId = parent.Id
	}

	var childId int64
	for _, child := range parent.Folders {
		err = d.db.QueryRow(
			`INSERT INTO `+FOLDER_TABLE+`(name) VALUES($1) RETURNING id`, child.Name).Scan(&childId)
		if err != nil {
			return err
		}
		child.Id = childId
		_, err = d.db.Query(
			`INSERT INTO `+FOLDER_CHILDREN_TABLE+`(parent, child) VALUES($1, $2)`, parent.Id, child.Id)
		if err != nil {
			return err
		}
		if err = d.importChildren(child); err != nil {
			return err
		}
	}
	return err
}
