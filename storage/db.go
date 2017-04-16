package storage

import (
	"database/sql"
	"github.com/jrupac/goliath/models"
	_ "github.com/lib/pq"
)

const (
	DIALECT               = "postgres"
	FOLDER_TABLE          = "Folder"
	FOLDER_CHILDREN_TABLE = "FolderChildren"
	FEED_TABLE            = "Feed"
	ARTICLE_TABLE         = "Article"
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
	var articleId int
	err := d.db.QueryRow(
		`INSERT INTO `+ARTICLE_TABLE+`
		(feed, folder, title, summary, content, link, date, read)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		a.FeedId, a.FolderId, a.Title, a.Summary, a.Content, a.Link, a.Date, a.Read).Scan(&articleId)
	if err != nil {
		return err
	}
	a.Id = articleId
	return nil
}

func (d *Database) GetAllFeeds() ([]models.Feed, error) {
	feeds := []models.Feed{}
	rows, err := d.db.Query(`SELECT id, folder, title, description, url, text FROM ` + FEED_TABLE)
	if err != nil {
		return feeds, err
	}
	defer rows.Close()

	for rows.Next() {
		f := models.Feed{}
		if err = rows.Scan(&f.Id, &f.FolderId, &f.Title, &f.Description, &f.Url, &f.Text); err != nil {
			return feeds, err
		}
		feeds = append(feeds, f)
	}

	return feeds, err
}

func (d *Database) ImportOpml(opml *models.Opml) error {
	root := opml.Folders
	var rootId int
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
	var feedId int
	for _, f := range parent.Feed {
		err = d.db.QueryRow(
			`INSERT INTO `+FEED_TABLE+`(folder, title, description, url, text)
			VALUES($1, $2, $3, $4, $5) RETURNING id`,
			parent.Id, f.Title, f.Description, f.Url, f.Text).Scan(&feedId)
		if err != nil {
			return err
		}
		f.Id = feedId
		f.FolderId = parent.Id
	}

	var childId int
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
