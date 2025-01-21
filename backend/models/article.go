package models

import (
	"fmt"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/utils"
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

// GetContents tries to return a non-empty content field for this article.
func (a *Article) GetContents(serveParsed bool) string {
	var content string
	if serveParsed && a.Parsed != "" {
		log.V(2).Infof("Serving parsed content for title: %s", a.Title)
		content = a.Parsed
	} else if a.Content != "" {
		// The "content" field usually has more text, but is not always set.
		content = a.Content
	} else {
		content = a.Summary
	}
	return content
}

func (a *Article) DebugString() string {
	// Substring length to print out for string fields
	n := 100

	return fmt.Sprintf(
		"\n\t(Folder, Feed, Article): (%d, %d, %d)\n\tLink: %s\n\tTitle: %s\n\tSummary: %s\n\tContent: %s\n",
		a.FolderID, a.FeedID, a.ID, a.Link, utils.Substring(a.Title, n), utils.Substring(a.Summary, n), utils.Substring(a.Content, n))
}
