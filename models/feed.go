package models

import (
	"fmt"
	"golang.org/x/crypto/sha3"
	"time"
)

// Feed is a single source of articles.
type Feed struct {
	// Primary key
	ID       int64
	FolderID int64
	// Data fields
	Title       string
	Description string
	URL         string
	Link        string
	Latest      time.Time
}

// Hash returns a SHA256 hash of this object.
func (a *Feed) Hash() string {
	h := sha3.New256()
	h.Write([]byte(a.Title))
	h.Write([]byte(a.Description))
	h.Write([]byte(a.URL))
	h.Write([]byte(a.Link))
	return fmt.Sprintf("%x", h.Sum(nil))
}
