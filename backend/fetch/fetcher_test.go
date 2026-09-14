package fetch

import (
	"bytes"
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
	"github.com/jrupac/rss/v2"
	"net/http/httptest"
	"strings"
	"sync/atomic"
)

// roundTripFunc answers a client's requests without a network.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// clientReader returns the feed reader a fetcher builds around client.
func clientReader(t *testing.T, client *http.Client) *rss.Reader {
	t.Helper()
	r, err := newFeedReader(client)
	if err != nil {
		t.Fatalf("newFeedReader: %v", err)
	}
	return r
}

// stubReader returns a feed reader whose every request respond answers.
func stubReader(t *testing.T, respond roundTripFunc) *rss.Reader {
	t.Helper()
	return clientReader(t, &http.Client{Transport: respond})
}

// sampleFeedReader returns a feed reader that answers every request with the
// sample feed.
func sampleFeedReader(t *testing.T) *rss.Reader {
	t.Helper()
	return stubReader(t, func(r *http.Request) (*http.Response, error) {
		file, err := os.Open("testdata/sample_feed.xml")
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: file, Request: r}, nil
	})
}

func loadTestData(t *testing.T) *rss.Feed {
	t.Helper()
	content, err := os.ReadFile("testdata/sample_feed.xml")
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	feed, err := clientReader(t, http.DefaultClient).Parse("http://example.com/feed", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to parse test data: %v", err)
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
		db.OnGetArticlesForFeedForUser = func(u models.User, feedID models.FeedId) ([]models.Article, error) {
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

func fetchTask(user models.User, feedID models.FeedId) task {
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
			OnGetFeedForUser: func(_ models.User, feedID models.FeedId) (models.Feed, error) {
				return models.Feed{ID: feedID, URL: "http://example.com/feed", Latest: pastTime}, nil
			},
		}
		fetcher := Fetcher{
			d:        db,
			retCache: cache.NewMockRetrievalCache(),
			finder:   &mockIconFinder{},
			reader:   sampleFeedReader(t),
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

// A fetch stores what it accepted in one call, and moves the feed's latest
// time once, to the newest of it. An item a document repeats is stored once.
func TestProcessUserFeedItemsStoresTogether(t *testing.T) {
	items := loadTestData(t).Items
	pastTime, _ := time.Parse(time.RFC3339, "2025-01-01T00:00:00Z")
	var calls int
	var latest []time.Time
	db := &storage.MockDB{
		OnInsertArticlesForUser: func(_ models.User, _ models.FeedId, articles []models.Article) (int, error) {
			calls++
			return len(articles), nil
		},
		OnUpdateLatestTimeForFeedForUser: func(_ models.User, _ models.FeedId, l time.Time) error {
			latest = append(latest, l)
			return nil
		},
	}
	fetcher := Fetcher{d: db, retCache: cache.NewMockRetrievalCache()}
	feed := &models.Feed{ID: 1, Latest: pastTime}

	if err := fetcher.processUserFeedItems(context.Background(), models.User{UserId: "u"}, feed, append(items, items[0])); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(db.InsertedArticles) != 2 {
		t.Errorf("%d calls storing %d articles, want 1 storing 2", calls, len(db.InsertedArticles))
	}
	want := time.Date(2025, 10, 12, 11, 0, 0, 0, time.UTC)
	if len(latest) != 1 || !latest[0].Equal(want) {
		t.Errorf("latest time updated to %v, want once to %s", latest, want)
	}
}

// An article the database refuses costs only itself: the batch is retried an
// article at a time, and the feed's latest time stops short of the refused
// one so that the next fetch tries it again.
func TestProcessUserFeedItemsRetriesARefusedBatchOneAtATime(t *testing.T) {
	items := loadTestData(t).Items
	pastTime, _ := time.Parse(time.RFC3339, "2025-01-01T00:00:00Z")
	user := models.User{UserId: "u"}
	var latest []time.Time
	db := &storage.MockDB{
		OnInsertArticlesForUser: func(_ models.User, _ models.FeedId, articles []models.Article) (int, error) {
			for _, a := range articles {
				if a.Title == "Test Article 2" {
					return 0, errors.New("refused")
				}
			}
			return len(articles), nil
		},
		OnUpdateLatestTimeForFeedForUser: func(_ models.User, _ models.FeedId, l time.Time) error {
			latest = append(latest, l)
			return nil
		},
	}
	retCache := cache.NewMockRetrievalCache()
	fetcher := Fetcher{d: db, retCache: retCache}
	feed := &models.Feed{ID: 1, Latest: pastTime}

	if err := fetcher.processUserFeedItems(context.Background(), user, feed, items); err != nil {
		t.Fatal(err)
	}
	if len(db.InsertedArticles) != 1 || db.InsertedArticles[0].Title != "Test Article 1" {
		t.Fatalf("stored %+v, want only Test Article 1", db.InsertedArticles)
	}
	want := time.Date(2025, 10, 12, 10, 0, 0, 0, time.UTC)
	if len(latest) != 1 || !latest[0].Equal(want) {
		t.Errorf("latest time updated to %v, want once to the stored article's %s", latest, want)
	}
	if !retCache.Lookup(user, feed.ID, db.InsertedArticles[0].Hash()) {
		t.Error("the stored article is not in the retrieval cache")
	}
	if refused := processItem(feed, items[1]); retCache.Lookup(user, feed.ID, refused.Hash()) {
		t.Error("the refused article is in the retrieval cache, so the next fetch would skip it")
	}
}

// The feed is read afresh for every fetch, so one unsubscribed from since the
// last is dropped rather than fetched.
func TestFetchFeedDropsAFeedNoLongerSubscribedTo(t *testing.T) {
	fetcher := Fetcher{
		d: &storage.MockDB{},
		reader: stubReader(t, func(*http.Request) (*http.Response, error) {
			t.Error("fetched a feed no longer subscribed to")
			return nil, errors.New("unreachable")
		}),
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
		OnGetFeedForUser: func(_ models.User, feedID models.FeedId) (models.Feed, error) {
			return models.Feed{ID: feedID, URL: "http://example.com/feed", Latest: pastTime}, nil
		},
		OnInsertArticlesForUser: func(models.User, models.FeedId, []models.Article) (int, error) {
			inserts++
			return 0, storage.ErrFeedGone
		},
		OnUpdateEstimatedRefreshIntervalForFeedForUser: func(models.User, models.FeedId, int) error {
			t.Error("rescheduled a feed that is gone")
			return nil
		},
	}
	fetcher := Fetcher{d: db, retCache: cache.NewMockRetrievalCache(), finder: &mockIconFinder{}, reader: sampleFeedReader(t)}

	if o := fetcher.fetchFeed(fetchTask(models.User{UserId: "u"}, 1)); !o.gone {
		t.Errorf("outcome = %+v, want the feed reported gone", o)
	}
	if inserts != 1 {
		t.Errorf("attempted %d inserts, want to stop after the first", inserts)
	}
}

func TestFetchFeedBacksOffOnFailure(t *testing.T) {
	db := &storage.MockDB{
		OnGetFeedForUser: func(_ models.User, feedID models.FeedId) (models.Feed, error) {
			return models.Feed{ID: feedID, URL: "http://example.com/feed"}, nil
		},
	}
	fetcher := Fetcher{d: db, reader: stubReader(t, func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})}

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

// A fetch in flight follows its task, so a worker is free as soon as the feed
// is unscheduled or fetching stops, rather than once the request times out.
func TestFetchFeedAbandonsARequestWhenItsTaskIsCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()

	db := &storage.MockDB{
		OnGetFeedForUser: func(_ models.User, feedID models.FeedId) (models.Feed, error) {
			return models.Feed{ID: feedID, URL: server.URL}, nil
		},
	}
	fetcher := Fetcher{d: db, reader: clientReader(t, server.Client())}

	ctx, cancel := context.WithCancel(context.Background())
	tk := fetchTask(models.User{UserId: "u"}, 1)
	tk.ctx = ctx
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	fetcher.fetchFeed(tk)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("fetch returned after %s, want it abandoned once its task was cancelled", elapsed)
	}
}

// Every outbound request carries the same identity, so that a publisher
// deciding how to treat Goliath is deciding about one client. A second
// hardcoded string somewhere would quietly split that in two.
func TestUserAgentIsSentOnFeedFetches(t *testing.T) {
	var got, accept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		accept = r.Header.Get("Accept")
		_, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title></channel></rss>`))
	}))
	defer server.Close()

	allowed, err := utils.NewAddressAllowlist([]string{strings.TrimPrefix(server.URL, "http://")})
	if err != nil {
		t.Fatalf("NewAddressAllowlist: %v", err)
	}

	if _, err = clientReader(t, newFeedClient(allowed)).Fetch(context.Background(), server.URL); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !strings.Contains(accept, "application/rss+xml") {
		t.Errorf("Accept = %q, which does not ask for a feed", accept)
	}

	if got != UserAgent() {
		t.Errorf("User-Agent = %q, want %q", got, UserAgent())
	}
	if !strings.Contains(got, "Goliath") {
		t.Errorf("User-Agent %q does not identify this server", got)
	}
}

// A favicon lookup goes wherever the feed's document points, so it is guarded
// like a feed fetch. The refusal happens in the dialer, so nothing is sent.
func TestIconLookupsRefuseInternalAddresses(t *testing.T) {
	_, err := newIconClient().Get("http://169.254.169.254/latest/meta-data/")
	if !errors.Is(err, utils.ErrBlockedAddress) {
		t.Errorf("err = %v, want %v", err, utils.ErrBlockedAddress)
	}

	fetcher, err := New(&storage.MockDB{}, &cache.MockRetrievalCache{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if icons, _ := fetcher.finder.FetchIcons("http://169.254.169.254/"); len(icons) != 0 {
		t.Errorf("found %d icons at a link-local address", len(icons))
	}
}

func TestUserAgentIsSentOnIconFetches(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
	}))
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: userAgentTransport{http.DefaultTransport}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	_ = resp.Body.Close()

	if got != UserAgent() {
		t.Errorf("User-Agent = %q, want %q", got, UserAgent())
	}
	if ua := req.Header.Get("User-Agent"); ua != "" {
		t.Errorf("the caller's request was modified: User-Agent %q", ua)
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

// Dates in the looser formats feeds use are read, so that items carrying them
// keep the date they were published rather than a made-up one.
func TestFeedReaderReadsExtraDateFormats(t *testing.T) {
	reader := clientReader(t, http.DefaultClient)
	for _, date := range []string{
		"2025-10-12",
		"Sun, 12 Oct 2025",
		"Sunday, 12 Oct 2025 10:00:00 UTC",
		"Sun, 12 Oct 2025 10:00:00 UTC",
		"Thu, 2 Oct 2025 10:00:00 UTC",
	} {
		doc := `<rss version="2.0"><channel><title>t</title><item><title>i</title>` +
			`<link>http://example.com/i</link><pubDate>` + date + `</pubDate></item></channel></rss>`
		feed, err := reader.Parse("http://example.com/feed", strings.NewReader(doc))
		if err != nil {
			t.Fatalf("%q: %v", date, err)
		}
		if len(feed.Items) != 1 || feed.Items[0].Date.IsZero() {
			t.Errorf("%q was not read as a date", date)
		}
	}
}

// Only an interval the feed declared schedules its next fetch. A feed naming
// none is left to the adaptive estimate, whatever the parser's default.
func TestDeclaredNextFetchIsOnlyWhatTheFeedDeclared(t *testing.T) {
	reader := clientReader(t, http.DefaultClient)
	for _, tc := range []struct {
		name     string
		doc      string
		declared bool
	}{
		{"nothing declared", `<rss version="2.0"><channel><title>t</title></channel></rss>`, false},
		{"a ttl", `<rss version="2.0"><channel><title>t</title><ttl>60</ttl></channel></rss>`, true},
		{"a syndication period", `<rss version="2.0" xmlns:sy="http://purl.org/rss/1.0/modules/syndication/"><channel><title>t</title>` +
			`<sy:updatePeriod>hourly</sy:updatePeriod><sy:updateFrequency>1</sy:updateFrequency></channel></rss>`, false},
	} {
		feed, err := reader.Parse("http://example.com/feed", strings.NewReader(tc.doc))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := !declaredNextFetch(feed).IsZero(); got != tc.declared {
			t.Errorf("%s: declared = %t, want %t", tc.name, got, tc.declared)
		}
	}
}

// A zone given by its abbreviation is read at its offset, whatever zone this
// server runs in.
func TestFeedReaderReadsZoneAbbreviations(t *testing.T) {
	reader := clientReader(t, http.DefaultClient)
	for date, want := range map[string]string{
		"Sun, 12 Oct 2025 10:00:00 GMT": "2025-10-12T10:00:00Z",
		"Sun, 12 Oct 2025 10:00:00 EDT": "2025-10-12T14:00:00Z",
		"Sun, 12 Oct 2025 10:00:00 PDT": "2025-10-12T17:00:00Z",
	} {
		doc := `<rss version="2.0"><channel><title>t</title><item><title>i</title>` +
			`<link>http://example.com/i</link><pubDate>` + date + `</pubDate></item></channel></rss>`
		feed, err := reader.Parse("http://example.com/feed", strings.NewReader(doc))
		if err != nil {
			t.Fatalf("%q: %v", date, err)
		}
		if got := feed.Items[0].Date.UTC().Format(time.RFC3339); got != want {
			t.Errorf("%q read as %s, want %s", date, got, want)
		}
	}
}

// A fetch sends the validators the last one got back, and a feed unchanged
// since is a success that stores nothing and leaves its metadata due.
func TestFetchFeedMakesConditionalRequests(t *testing.T) {
	var requests, conditional atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("If-None-Match") == `"v1"` {
			conditional.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		body, err := os.ReadFile("testdata/sample_feed.xml")
		if err != nil {
			t.Error(err)
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	pastTime, _ := time.Parse(time.RFC3339, "2025-01-01T00:00:00Z")
	db := &storage.MockDB{
		OnGetFeedForUser: func(_ models.User, feedID models.FeedId) (models.Feed, error) {
			return models.Feed{ID: feedID, URL: server.URL, Latest: pastTime}, nil
		},
	}
	fetcher := Fetcher{d: db, retCache: cache.NewMockRetrievalCache(), finder: &mockIconFinder{},
		reader: clientReader(t, server.Client())}

	first := fetcher.fetchFeed(fetchTask(models.User{UserId: "u"}, 1))
	if first.failures != 0 || first.state.etag != `"v1"` {
		t.Fatalf("first fetch: %+v, want a success carrying the ETag", first)
	}

	tk := fetchTask(models.User{UserId: "u"}, 1)
	tk.state = first.state
	tk.refreshMetadata = true
	second := fetcher.fetchFeed(tk)

	if requests.Load() != 2 || conditional.Load() != 1 {
		t.Errorf("%d requests, %d conditional; want the second to be conditional", requests.Load(), conditional.Load())
	}
	if second.gone || second.failures != 0 || !second.next.After(time.Now()) {
		t.Errorf("second fetch: %+v, want a success scheduled in the future", second)
	}
	if second.refreshedMetadata || db.UpdateFeedMetadataForUserCalled {
		t.Error("refreshed metadata without a document to refresh it from")
	}
	if second.state != first.state {
		t.Errorf("state after a 304 = %+v, want %+v kept", second.state, first.state)
	}
	if len(db.InsertedArticles) != 2 {
		t.Errorf("stored %d articles, want only the first fetch's 2", len(db.InsertedArticles))
	}
}

// A server that says when to come back is not asked again sooner.
func TestFetchFeedWaitsAsLongAsTheServerAsks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7200")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	db := &storage.MockDB{
		OnGetFeedForUser: func(_ models.User, feedID models.FeedId) (models.Feed, error) {
			return models.Feed{ID: feedID, URL: server.URL}, nil
		},
	}
	fetcher := Fetcher{d: db, reader: clientReader(t, server.Client())}

	start := time.Now()
	o := fetcher.fetchFeed(fetchTask(models.User{UserId: "u"}, 1))

	if o.gone || o.failures != 1 {
		t.Fatalf("outcome = %+v, want a failure", o)
	}
	if want := start.Add(2 * time.Hour); o.next.Before(want) {
		t.Errorf("next fetch at %s, want no earlier than %s", o.next, want)
	}
}
