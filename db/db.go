package db

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

const DIALECT = "postgres"

func Open(dbPath string) (*gorm.DB, error) {
	d, err := gorm.Open(DIALECT, dbPath)
	if err != nil {
		return nil, err
	}

	d.AutoMigrate(&Folder{}, &Feed{}, &Article{})
	return d, nil
}
