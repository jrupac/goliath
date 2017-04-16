package models

import "hash/fnv"

type Feed struct {
	Id       int64
	FolderId int64

	Title       string
	Description string
	Url         string
	Text        string
}

func (a *Feed) Hash() int64 {
	h := fnv.New64()
	h.Write([]byte(a.Title))
	h.Write([]byte(a.Description))
	h.Write([]byte(a.Url))
	h.Write([]byte(a.Text))
	return int64(h.Sum64())
}
