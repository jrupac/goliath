package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jrupac/goliath/fetch"
	"github.com/jrupac/goliath/jsonsafe"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
)

// decodeResponse decodes a JSON response body keeping numbers as written, so
// that a test can tell a JSON number from a string holding the same digits.
func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", got, http.StatusOK, w.Body)
	}
	dec := json.NewDecoder(bytes.NewReader(w.Body.Bytes()))
	dec.UseNumber()
	var res map[string]any
	if err := dec.Decode(&res); err != nil {
		t.Fatalf("decode %s: %v", w.Body, err)
	}
	return res
}

// Item contents carry every ID as a string, Unix seconds as numbers, and the
// finer timestamps as strings, which is how clients decode each of them.
func TestStreamItemsContentsDescribesEachArticle(t *testing.T) {
	const id = models.ArticleId(1<<60 + 1)
	date := time.Unix(1725580800, 123456000)
	mockDB := &storage.MockDB{
		OnGetArticlesForUser: func(_ models.User, ids []models.ArticleId) ([]models.Article, error) {
			if !reflect.DeepEqual(ids, []models.ArticleId{id}) {
				t.Errorf("asked for articles %v, want [%d]", ids, id)
			}
			return []models.Article{{
				ID: id, FeedID: 7, Title: "Title", Summary: "<p>Summary</p>",
				Link: "https://example.invalid/a", Date: date,
			}}, nil
		},
		OnGetFeedsPerFolderForUser: func(models.User) (map[models.FolderId][]models.FeedId, error) {
			return map[models.FolderId][]models.FeedId{3: {7}}, nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleStreamItemsContents(w, subscriptionRequest(user,
		"/greader/reader/api/0/stream/items/contents",
		url.Values{"i": {fmt.Sprintf("%x", int64(id))}}), user)

	res := decodeResponse(t, w)
	if res["id"] != readingListStreamId {
		t.Errorf("id = %v, want %q", res["id"], readingListStreamId)
	}
	if _, ok := res["updated"].(json.Number); !ok {
		t.Errorf("updated = %#v, want a number", res["updated"])
	}
	items, _ := res["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %v, want one", res["items"])
	}
	want := map[string]any{
		"id":            greaderArticleId(id),
		"title":         "Title",
		"published":     json.Number("1725580800"),
		"crawlTimeMsec": "1725580800123",
		"timestampUsec": "1725580800123456",
		"categories":    []any{readingListStreamId, greaderFeedId(7), greaderFolderId(3)},
		"canonical":     []any{map[string]any{"href": "https://example.invalid/a"}},
		"alternate":     []any{map[string]any{"href": "https://example.invalid/a"}},
		"summary":       map[string]any{"content": "<p>Summary</p>"},
		"origin":        map[string]any{"streamId": greaderFeedId(7), "title": "", "htmlUrl": ""},
	}
	if got := items[0]; !reflect.DeepEqual(got, want) {
		t.Errorf("item =\n  %#v\nwant\n  %#v", got, want)
	}
}

func TestQuickAddSubscribesToANewFeed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0"><channel>`+
			`<title>New Feed</title><link>https://example.invalid/</link>`+
			`<item><title>First</title><link>https://example.invalid/1</link></item>`+
			`</channel></rss>`)
	}))
	defer ts.Close()
	t.Cleanup(fetch.SetDiscoverClientForTesting(ts.Client()))

	const id = models.FeedId(1<<60 + 2)
	feedURL := ts.URL + "/feed.xml"
	var inserted models.Feed
	mockDB := &storage.MockDB{
		OnGetFeedByUrlForUser: func(models.User, string) (models.Feed, error) {
			return models.Feed{}, sql.ErrNoRows
		},
		OnRestoreFeedByUrlForUser: func(models.User, string) (models.Feed, error) {
			return models.Feed{}, sql.ErrNoRows
		},
		OnInsertFeedForUser: func(_ models.User, f models.Feed, _ models.FolderId) (models.FeedId, error) {
			inserted = f
			return id, nil
		},
	}
	subs := &recordingSubscriptions{}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB, subs: subs}.handleQuickAdd(w, subscriptionRequest(user,
		"/greader/reader/api/0/subscription/quickadd", url.Values{"quickadd": {feedURL}}), user)

	want := map[string]any{
		"query":      feedURL,
		"numResults": json.Number("1"),
		"streamId":   greaderFeedId(id),
		"streamName": "New Feed",
	}
	if got := decodeResponse(t, w); !reflect.DeepEqual(got, want) {
		t.Errorf("response = %#v, want %#v", got, want)
	}
	if inserted.Title != "New Feed" || inserted.URL != feedURL {
		t.Errorf("inserted %+v, want the discovered feed", inserted)
	}
	if !reflect.DeepEqual(subs.scheduled, []models.FeedId{id}) {
		t.Errorf("scheduled %v, want [%d]", subs.scheduled, id)
	}
}

// An article parsed before is served as stored, without fetching it again.
func TestParseFullArticleServesWhatWasParsedBefore(t *testing.T) {
	old := *serveParsedArticles
	*serveParsedArticles = true
	t.Cleanup(func() { *serveParsedArticles = old })
	mockDB := &storage.MockDB{
		OnGetArticlesForUser: func(models.User, []models.ArticleId) ([]models.Article, error) {
			return []models.Article{{ID: 12345, Link: "https://example.invalid/a", Parsed: "<p>Stored</p>"}}, nil
		},
		OnUpdateArticleParsedContentForUser: func(models.User, models.ArticleId, string) error {
			t.Error("stored the article again")
			return nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleParseFullArticle(w, subscriptionRequest(user,
		"/greader/ext/parse-full-article", url.Values{"i": {"3039"}}), user)

	want := map[string]any{"content": "<p>Stored</p>"}
	if got := decodeResponse(t, w); !reflect.DeepEqual(got, want) {
		t.Errorf("response = %#v, want %#v", got, want)
	}
}

// A response a client could misread is refused with a server error, and none
// of it is sent: a type holding an integer that is neither quoted nor a
// jsonsafe.Int, and a jsonsafe.Int beyond what a client reads back exactly.
func TestReturnSuccessRefusesWhatAClientCouldMisread(t *testing.T) {
	type rawID struct {
		ID int64 `json:"id"`
	}
	for name, tc := range map[string]struct {
		resp   any
		digits string
	}{
		"unquoted integer": {rawID{ID: 1<<60 + 1}, "1152921504606846977"},
		"large jsonsafe.Int": {
			greaderStreamItemsContents{Id: "kept-out", Updated: jsonsafe.MaxInt + 1},
			"9007199254740992",
		},
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			GReader{}.returnSuccess(w, tc.resp)
			if w.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
			}
			if body := w.Body.String(); strings.Contains(body, tc.digits) || strings.Contains(body, "kept-out") {
				t.Errorf("body = %q, want none of the response", body)
			}
		})
	}
}
