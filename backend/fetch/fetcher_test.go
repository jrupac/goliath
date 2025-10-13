package fetch

import (
	"context"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jrupac/goliath/cache"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/rss"
)

// mockFetchFunc is a mock implementation of rss.FetchFunc for testing.
var mockFetchFunc = func(url string) (*http.Response, error) {
	file, err := os.Open("testdata/sample_feed.xml")
	if err != nil {
		return nil, err
	}
	return &http.Response{Body: file}, nil
}

func loadTestData(t *testing.T) *rss.Feed {
	t.Helper()
	content, err := os.ReadFile("testdata/sample_feed.xml")
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	feed, err := rss.Parse(content)
	if err != nil {
		t.Fatalf("Failed to parse test data: %v", err)
	}

	// Manually set DateValid to true as rss.Parse does not do this.
	for _, item := range feed.Items {
		item.DateValid = true
	}

	return feed
}

func TestProcessUserFeedItems(t *testing.T) {
	testFeedData := loadTestData(t)
	user := models.User{UserId: "test-user"}
	// Use a fixed past time for the test to make date comparisons predictable.
	pastTime, _ := time.Parse(time.RFC3339, "2025-01-01T00:00:00Z")

	t.Run("processes new articles", func(t *testing.T) {
		feed := &models.Feed{ID: 1, Latest: pastTime}
		db := &storage.MockDB{}
		fetcher := Fetcher{d: db, retCache: cache.NewMockRetrievalCache()}

		fetcher.processUserFeedItems(context.Background(), user, feed, testFeedData.Items)

		if len(db.InsertedArticles) != 2 {
			t.Errorf("expected 2 articles to be inserted, got %d", len(db.InsertedArticles))
		}

		if db.InsertedArticles[0].Title != "Test Article 1" {
			t.Errorf("unexpected title for first article: %s", db.InsertedArticles[0].Title)
		}
	})

	t.Run("filters articles that are too old", func(t *testing.T) {
		db := &storage.MockDB{}
		fetcher := Fetcher{d: db, retCache: cache.NewMockRetrievalCache()}
		// Set feed's latest time to be after the articles in the test data
		futureTime, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
		futureFeed := &models.Feed{ID: 1, Latest: futureTime}

		fetcher.processUserFeedItems(context.Background(), user, futureFeed, testFeedData.Items)

		if len(db.InsertedArticles) != 0 {
			t.Errorf("expected 0 articles to be inserted, got %d", len(db.InsertedArticles))
		}
	})

	t.Run("marks new article as read if similar existing are all read", func(t *testing.T) {
		feed := &models.Feed{ID: 1, Latest: pastTime}
		db := &storage.MockDB{}
		// Override GetArticlesForFeedForUser to return a similar, read article
		*strictDedup = true
		defer func() { *strictDedup = true }() // Restore
		db.OnGetArticlesForFeedForUser = func(u models.User, feedID int64) ([]models.Article, error) {
			return []models.Article{{
				Link: "http://example.com/article1", // Same link as first test article
				Read: true,
			}}, nil
		}

		fetcher := Fetcher{d: db, retCache: cache.NewMockRetrievalCache()}

		fetcher.processUserFeedItems(context.Background(), user, feed, testFeedData.Items)

		if len(db.InsertedArticles) != 2 {
			t.Fatalf("expected 2 articles to be inserted, got %d", len(db.InsertedArticles))
		}

		// The first article should be marked as read, the second should not
		foundArticle1 := false
		for _, article := range db.InsertedArticles {
			if article.Link == "http://example.com/article1" {
				foundArticle1 = true
				if !article.Read {
					t.Error("expected article 1 to be marked as read")
				}
			}
		}
		if !foundArticle1 {
			t.Error("did not find article 1 in inserted articles")
		}
	})
}

func TestFetchUserFeed(t *testing.T) {
	user := models.User{UserId: "test-user"}
	feed := models.Feed{ID: 1, URL: "http://example.com/feed"}
	db := &storage.MockDB{}

	fetcher := Fetcher{
		d:         db,
		retCache:  cache.NewMockRetrievalCache(),
		finder:    &mockIconFinder{},
		fetchFunc: mockFetchFunc,
	}

	// Use a context that we can cancel to stop the infinite loop in fetchUserFeed.
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	go fetcher.fetchUserFeed(ctx, &wg, user, feed)

	// Wait a moment for the first fetch to complete, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()
	wg.Wait()

	if !db.UpdateFeedMetadataForUserCalled {
		t.Error("expected UpdateFeedMetadataForUser to be called")
	}
	if len(db.InsertedArticles) == 0 {
		t.Error("expected articles to be inserted")
	}
}

func TestFetcher_PauseResume(t *testing.T) {
	user := models.User{UserId: "test-user"}
	db := &storage.MockDB{
		GetAllUsersCalled:  make(chan bool, 1),
		ProcessItemsCalled: make(chan bool, 1),
		OnGetAllUsers: func() ([]models.User, error) {
			return []models.User{user}, nil
		},
		OnGetAllFeedsForUser: func(u models.User) ([]models.Feed, error) {
			return []models.Feed{{ID: 1, URL: "http://example.com/feed"}}, nil
		},
	}

	retCache := cache.NewMockRetrievalCache()
	fetcher := New(db, retCache)
	fetcher.fetchFunc = mockFetchFunc

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cleanup

	go fetcher.Start(ctx)

	// 1. Wait for the first fetch to start
	select {
	case <-db.GetAllUsersCalled:
		// This is good, means the fetch loop started
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for fetcher to start")
	}

	// 2. Wait for the first set of items to be processed
	select {
	case <-db.ProcessItemsCalled:
		// This is good, means the first fetch completed
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for first fetch to complete")
	}

	// 3. Pause the fetcher
	t.Log("Pausing fetcher...")
	Pause()
	t.Log("Fetcher paused.")

	// 4. Resume the fetcher
	t.Log("Resuming fetcher...")
	Resume()
	t.Log("Fetcher resumed.")

	// 5. Wait for the second fetch to start
	select {
	case <-db.GetAllUsersCalled:
		// This is good, means the fetch loop has resumed
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for fetcher to resume")
	}
}
