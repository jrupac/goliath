package models

import (
	"fmt"
	"golang.org/x/crypto/sha3"
	"time"
)

// Article is a single fetched article.
type Article struct {
	// Primary key
	ID       int64
	FeedID   int64
	FolderID int64
	// Data fields
	Title     string
	Summary   string
	Content   string
	Parsed    string
	Link      string
	Read      bool
	Date      time.Time
	Retrieved time.Time
	// Metadata
	SyntheticDate bool
}

// Hash returns a SHA256 hash of this object.
func (a *Article) Hash() string {
	h := sha3.New256()
	h.Write([]byte(a.Title))
	h.Write([]byte(a.Summary))
	h.Write([]byte(a.Content))
	h.Write([]byte(a.Parsed))
	h.Write([]byte(a.Link))
	if !a.SyntheticDate {
		h.Write([]byte(a.Date.String()))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
