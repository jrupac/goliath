package models

import "time"

type Article struct {
	Id       int
	FeedId   int
	FolderId int

	Title   string
	Summary string
	Content string
	Link    string
	Date    time.Time
	Read    bool
}
