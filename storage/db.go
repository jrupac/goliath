package storage

import (
	"database/sql"
	"github.com/jrupac/goliath/models"
	_ "github.com/lib/pq"
)

const DIALECT = "postgres"

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

func (d *Database) ImportOpml(opml *models.Opml) error {
	root := opml.Folders
	var rootId int
	err := d.db.QueryRow(
		`INSERT INTO Folder(name) VALUES($1) RETURNING id`, root.Name).Scan(&rootId)
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
			`INSERT INTO Feed(folder, title, description, url, text)
			VALUES($1, $2, $3, $4, $5) RETURNING id`,
			parent.Id, f.Title, f.Description, f.Url, f.Text).Scan(&feedId)
		if err != nil {
			return err
		}
		f.Id = feedId
	}

	var childId int
	for _, child := range parent.Folders {
		err = d.db.QueryRow(
			`INSERT INTO Folder(name) VALUES($1) RETURNING id`, child.Name).Scan(&childId)
		if err != nil {
			return err
		}
		child.Id = childId
		_, err = d.db.Query(
			`INSERT INTO FolderChildren(parent, child) VALUES($1, $2)`, parent.Id, child.Id)
		if err != nil {
			return err
		}
		if err = d.importChildren(child); err != nil {
			return err
		}
	}
	return err
}
