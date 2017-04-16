package models

import (
	"hash/fnv"
	"time"
)

type Article struct {
	Id       int64
	FeedId   int64
	FolderId int64

	Title   string
	Summary string
	Content string
	Link    string
	Date    time.Time
	Read    bool
}

func (a *Article) Hash() int64 {
	h := fnv.New64()
	h.Write([]byte(a.Title))
	h.Write([]byte(a.Summary))
	h.Write([]byte(a.Content))
	h.Write([]byte(a.Link))
	h.Write([]byte(a.Date.String()))
	return int64(h.Sum64())
}
