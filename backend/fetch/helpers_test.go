package fetch

import (
	"context"
	"errors"
	"testing"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/rss"
	"github.com/mat/besticon/v3/besticon"
)

// A valid 1x1 PNG for testing image decoding.
var png1x1 = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82}

// mockIconFinder is a mock implementation of the IconFinder interface for testing.
type mockIconFinder struct {
	icons []besticon.Icon
	err   error
}

func (m *mockIconFinder) FetchIcons(url string) ([]besticon.Icon, error) {
	return m.icons, m.err
}

func TestUpdateFeedMetadataForUser(t *testing.T) {
	db := &storage.MockDB{}
	fetcher := Fetcher{d: db}

	user := models.User{UserId: "test-user"}

	t.Run("valid fields", func(t *testing.T) {
		modelFeed := &models.Feed{ID: 1}
		rssFeed := &rss.Feed{
			Title:       "Test Title",
			Description: "Test Description",
			Link:        "http://example.com",
		}

		fetcher.updateFeedMetadataForUser(context.Background(), user, modelFeed, rssFeed)

		if modelFeed.Title != "Test Title" {
			t.Errorf("expected title to be %q, got %q", "Test Title", modelFeed.Title)
		}
		if modelFeed.Description != "Test Description" {
			t.Errorf("expected description to be %q, got %q", "Test Description", modelFeed.Description)
		}
		if modelFeed.Link != "http://example.com" {
			t.Errorf("expected link to be %q, got %q", "http://example.com", modelFeed.Link)
		}
		if !db.UpdateFeedMetadataForUserCalled {
			t.Error("expected UpdateFeedMetadataForUser to be called on the database")
		}
	})

	t.Run("non-absolute link", func(t *testing.T) {
		modelFeed := &models.Feed{ID: 1}
		rssFeed := &rss.Feed{
			Title:       "Test Title",
			Description: "Test Description",
			Link:        "/",
		}

		fetcher.updateFeedMetadataForUser(context.Background(), user, modelFeed, rssFeed)

		if modelFeed.Title != "Test Title" {
			t.Errorf("expected title to be %q, got %q", "Test Title", modelFeed.Title)
		}
		if modelFeed.Description != "Test Description" {
			t.Errorf("expected description to be %q, got %q", "Test Description", modelFeed.Description)
		}
		if modelFeed.Link != "" {
			t.Errorf("expected link to be empty, got %q", modelFeed.Link)
		}
		if !db.UpdateFeedMetadataForUserCalled {
			t.Error("expected UpdateFeedMetadataForUser to be called on the database")
		}
	})
}

func TestTryIconFetch(t *testing.T) {
	t.Run("no link", func(t *testing.T) {
		fetcher := Fetcher{}
		_, _, err := fetcher.tryIconFetch("")
		if err == nil || err.Error() != "invalid URL" {
			t.Errorf("expected 'invalid URL' error, got %v", err)
		}
	})

	t.Run("fetch icons fails", func(t *testing.T) {
		finder := &mockIconFinder{err: errors.New("fetch failed")}
		fetcher := Fetcher{finder: finder}
		_, _, err := fetcher.tryIconFetch("http://example.com")
		if err == nil || err.Error() != "fetch failed" {
			t.Errorf("expected 'fetch failed' error, got %v", err)
		}
	})

	t.Run("no icons found", func(t *testing.T) {
		finder := &mockIconFinder{icons: []besticon.Icon{}}
		fetcher := Fetcher{finder: finder}
		_, _, err := fetcher.tryIconFetch("http://example.com")
		if err == nil || err.Error() != "no icons found" {
			t.Errorf("expected 'no icons found' error, got %v", err)
		}
	})

	t.Run("image decoding fails", func(t *testing.T) {
		icons := []besticon.Icon{{URL: "http://example.com/icon.ico", Format: "ico", ImageData: []byte("invalid data")}}
		finder := &mockIconFinder{icons: icons}
		fetcher := Fetcher{finder: finder}
		_, _, err := fetcher.tryIconFetch("http://example.com")
		if err == nil || err.Error() != "no suitable icons found" {
			t.Errorf("expected 'no suitable icons found' error, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		icons := []besticon.Icon{{URL: "http://example.com/icon.png", Format: "png", ImageData: png1x1}}
		finder := &mockIconFinder{icons: icons}
		fetcher := Fetcher{finder: finder}
		icon, img, err := fetcher.tryIconFetch("http://example.com")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if icon.URL != "http://example.com/icon.png" {
			t.Errorf("unexpected icon URL: %s", icon.URL)
		}
		if img == nil {
			t.Fatal("expected a decoded image, got nil")
		}
		if (*img).Bounds().Dx() != 1 || (*img).Bounds().Dy() != 1 {
			t.Errorf("expected 1x1 image, got %dx%d", (*img).Bounds().Dx(), (*img).Bounds().Dy())
		}
	})
}

func TestUpdateFeedFaviconForUser(t *testing.T) {
	user := models.User{UserId: "test-user"}
	feed := &models.Feed{ID: 1, Link: "http://example.com"}
	rssFeed := &rss.Feed{Link: "http://example.com", Image: &rss.Image{}}

	t.Run("no icon found", func(t *testing.T) {
		db := &storage.MockDB{}
		finder := &mockIconFinder{err: errors.New("not found")}
		fetcher := Fetcher{d: db, finder: finder}

		fetcher.updateFeedFaviconForUser(context.Background(), user, feed, rssFeed)

		if db.InsertFaviconForUserCalled {
			t.Error("expected InsertFaviconForUser not to be called")
		}
	})

	t.Run("icon found and inserted", func(t *testing.T) {
		db := &storage.MockDB{}
		icons := []besticon.Icon{{URL: "http://example.com/icon.png", Format: "png", ImageData: png1x1}}
		finder := &mockIconFinder{icons: icons}
		fetcher := Fetcher{d: db, finder: finder}

		fetcher.updateFeedFaviconForUser(context.Background(), user, feed, rssFeed)

		if !db.InsertFaviconForUserCalled {
			t.Error("expected InsertFaviconForUser to be called")
		}
	})
}
