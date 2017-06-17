package models

import (
	"fmt"
	"golang.org/x/crypto/sha3"
	"time"
)

type Feed struct {
	Id          int64
	FolderId    int64

	Title       string
	Description string
	Url         string
	Latest      time.Time
}

func (a *Feed) Hash() string {
	h := sha3.New256()
	h.Write([]byte(a.Title))
	h.Write([]byte(a.Description))
	h.Write([]byte(a.Url))
	return fmt.Sprintf("%x", h.Sum(nil))
}
