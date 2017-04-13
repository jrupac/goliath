package db

import (
	"database/sql"
	"github.com/jrupac/goliath/models"
	_ "github.com/lib/pq"
)

const DIALECT = "postgres"

type Database struct {
	d *sql.DB
}

func Open(dbPath string) (*Database, error) {
	database := new(Database)

	d, err := sql.Open(DIALECT, dbPath)
	if err != nil {
		return nil, err
	}

	database.d = d
	return database, nil
}

func (database *Database) Close() error {
	return database.d.Close()
}

func (database *Database) ImportOpml(opml *models.Opml) error {
	return nil
}
