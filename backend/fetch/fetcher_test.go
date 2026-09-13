package fetch

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jrupac/goliath/cache"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/jrupac/rss"
	"net/http/httptest"
	"strings"
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

func fetchTask(user models.User, feedID int64) task {
	return task{
		ctx:  context.Background(),
		key:  storage.UserFeedKey{UserID: user.UserId, FeedID: feedID},
		user: user,
	}
}

func TestFetchFeed(t *testing.T) {
	user := models.User{UserId: "test-user", Username: "test"}
	pastTime, _ := time.Parse(time.RFC3339, "2025-01-01T00:00:00Z")

	for _, refresh := range []bool{true, false} {
		db := &storage.MockDB{
			OnGetFeedForUser: func(_ models.User, feedID int64) (models.Feed, error) {
				return models.Feed{ID: feedID, URL: "http://example.com/feed", Latest: pastTime}, nil
			},
		}
		fetcher := Fetcher{
			d:         db,
			retCache:  cache.NewMockRetrievalCache(),
			finder:    &mockIconFinder{},
			fetchFunc: mockFetchFunc,
		}

		tk := fetchTask(user, 1)
		tk.failures = 2
		tk.refreshMetadata = refresh
		o := fetcher.fetchFeed(tk)

		if o.gone || o.failures != 0 {
			t.Errorf("refresh=%t: outcome = %+v, want a success", refresh, o)
		}
		if !o.next.After(time.Now()) {
			t.Errorf("refresh=%t: next fetch %s is not in the future", refresh, o.next)
		}
		if len(db.InsertedArticles) != 2 {
			t.Errorf("refresh=%t: inserted %d articles, want 2", refresh, len(db.InsertedArticles))
		}
		// Metadata is refreshed only when the scheduler says it is due.
		if o.refreshedMetadata != refresh || db.UpdateFeedMetadataForUserCalled != refresh {
			t.Errorf("refresh=%t: refreshed metadata = %t, wrote it = %t",
				refresh, o.refreshedMetadata, db.UpdateFeedMetadataForUserCalled)
		}
	}
}

// The feed is read afresh for every fetch, so one unsubscribed from since the
// last is dropped rather than fetched.
func TestFetchFeedDropsAFeedNoLongerSubscribedTo(t *testing.T) {
	fetcher := Fetcher{
		d: &storage.MockDB{},
		fetchFunc: func(string) (*http.Response, error) {
			t.Error("fetched a feed no longer subscribed to")
			return nil, errors.New("unreachable")
		},
	}

	if o := fetcher.fetchFeed(fetchTask(models.User{UserId: "u"}, 1)); !o.gone {
		t.Errorf("outcome = %+v, want the feed reported gone", o)
	}
}

// A feed unsubscribed from while it is being fetched turns the fetch's writes
// into no-ops. The fetch stops at the first one rather than carrying on and
// scheduling a feed that no longer exists.
func TestFetchFeedStopsWhenUnsubscribedFromMidFetch(t *testing.T) {
	pastTime, _ := time.Parse(time.RFC3339, "2025-01-01T00:00:00Z")
	var inserts int
	db := &storage.MockDB{
		OnGetFeedForUser: func(_ models.User, feedID int64) (models.Feed, error) {
			return models.Feed{ID: feedID, URL: "http://example.com/feed", Latest: pastTime}, nil
		},
		OnInsertArticleForUser: func(models.User, models.Article) error {
			inserts++
			return storage.ErrFeedGone
		},
		OnUpdateEstimatedRefreshIntervalForFeedForUser: func(models.User, int64, int) error {
			t.Error("rescheduled a feed that is gone")
			return nil
		},
	}
	fetcher := Fetcher{d: db, retCache: cache.NewMockRetrievalCache(), finder: &mockIconFinder{}, fetchFunc: mockFetchFunc}

	if o := fetcher.fetchFeed(fetchTask(models.User{UserId: "u"}, 1)); !o.gone {
		t.Errorf("outcome = %+v, want the feed reported gone", o)
	}
	if inserts != 1 {
		t.Errorf("attempted %d inserts, want to stop after the first", inserts)
	}
}

func TestFetchFeedBacksOffOnFailure(t *testing.T) {
	db := &storage.MockDB{
		OnGetFeedForUser: func(_ models.User, feedID int64) (models.Feed, error) {
			return models.Feed{ID: feedID, URL: "http://example.com/feed"}, nil
		},
	}
	fetcher := Fetcher{d: db, fetchFunc: func(string) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}}

	tk := fetchTask(models.User{UserId: "u"}, 1)
	tk.failures = 2
	start := time.Now()
	o := fetcher.fetchFeed(tk)

	if o.gone || o.failures != 3 {
		t.Fatalf("outcome = %+v, want a third consecutive failure", o)
	}
	if want := start.Add(fetcher.calculateFailureBackoff(3)); o.next.Before(want) {
		t.Errorf("next fetch at %s, want no earlier than %s", o.next, want)
	}
}

// Every outbound request carries the same identity, so that a publisher
// deciding how to treat Goliath is deciding about one client. A second
// hardcoded string somewhere would quietly split that in two.
func TestUserAgentIsSentOnFeedFetches(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title></channel></rss>`))
	}))
	defer server.Close()

	allowed, err := utils.NewAddressAllowlist([]string{strings.TrimPrefix(server.URL, "http://")})
	if err != nil {
		t.Fatalf("NewAddressAllowlist: %v", err)
	}

	resp, err := fetchFuncWithClient(newFeedClient(allowed))(server.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if got != UserAgent() {
		t.Errorf("User-Agent = %q, want %q", got, UserAgent())
	}
	if !strings.Contains(got, "Goliath") {
		t.Errorf("User-Agent %q does not identify this server", got)
	}
}

// Full-text extraction may need a different identity for sites that vary
// content by client, but says so explicitly rather than by carrying its own
// copy of the default.
func TestFullTextUserAgentDefaultsToTheSharedOne(t *testing.T) {
	if *fullTextUserAgent != "" {
		t.Errorf("fullTextUserAgent defaults to %q, want empty so it falls back to userAgent", *fullTextUserAgent)
	}
}
