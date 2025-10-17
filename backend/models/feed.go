package models

import (
	"fmt"
	"time"

	"golang.org/x/crypto/sha3"
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
func (f Feed) Hash() string {
	h := sha3.New256()
	h.Write([]byte(f.Title))
	h.Write([]byte(f.Description))
	h.Write([]byte(f.URL))
	h.Write([]byte(f.Link))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (f Feed) String() string {
	return fmt.Sprintf(
		"Feed{Folder:%d, ID:%d, Title:\"%s\", Link:\"%s\", URL:\"%s\"}",
		f.FolderID, f.ID, f.Title, f.Link, f.URL)
}
