package models

import (
	"fmt"
	"golang.org/x/crypto/sha3"
)

type Feed struct {
	Id       int64
	FolderId int64

	Title       string
	Description string
	Url         string
	Text        string
}

func (a *Feed) Hash() string {
	h := sha3.New256()
	h.Write([]byte(a.Title))
	h.Write([]byte(a.Description))
	h.Write([]byte(a.Url))
	h.Write([]byte(a.Text))
	return fmt.Sprintf("%x", h.Sum(nil))
}
