package db

import "github.com/jinzhu/gorm"

type Folder struct {
	gorm.Model

	Feeds  []Feed `xml:"ForeignKey:FeedId"`
	FeedId int

	Folders  []Folder `gorm:"ForeignKey:FolderId"`
	FolderId int
}

type Feed struct {
	gorm.Model

	Title       string
	Description string
	URL         string
	Text        string
}

type Article struct {
	gorm.Model
}
