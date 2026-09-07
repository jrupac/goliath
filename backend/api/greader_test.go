package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jrupac/goliath/fetch"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
)

func TestHandleParseFullArticle(t *testing.T) {
	// Enable serveParsedArticles flag for testing
	*serveParsedArticles = true

	// Set up mock HTTP server for the extracted article
	articleHTML := `
<!DOCTYPE html>
<html>
<head><title>Test Article</title></head>
<body>
	<article>
		<h1>Article Header</h1>
		<p>Here is a relative link: <a href="subpage/link">Relative Link</a></p>
		<p>Here is a relative image: <img src="images/photo.jpg" /></p>
		<script>alert("evil script")</script>
	</article>
</body>
</html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(articleHTML))
	}))
	defer ts.Close()

	// Override the HTTP client in the readability extractor
	fetch.SetClientForTesting(ts.Client())

	// Configure mock database
	mockDB := &storage.MockDB{
		OnGetArticlesForUser: func(u models.User, ids []int64) ([]models.Article, error) {
			if len(ids) == 1 && ids[0] == 12345 {
				return []models.Article{
					{
						ID:     12345,
						Title:  "Test Title",
						Link:   ts.URL + "/folder/article.html", // Base URL for relative resolution
						Parsed: "",                              // Empty to trigger extraction
					},
				}, nil
			}
			return nil, nil
		},
	}

	var savedParsedContent string
	mockDB.OnUpdateArticleParsedContentForUser = func(u models.User, articleID int64, parsed string) error {
		if articleID == 12345 {
			savedParsedContent = parsed
		}
		return nil
	}

	greader := GReader{d: mockDB}

	// Prepare request
	form := url.Values{}
	form.Add("T", "post_token")
	form.Add("i", "3039") // hex representation of 12345 is 3039 (12345 = 0x3039)
	req := httptest.NewRequest("POST", "/greader/ext/parse-full-article", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	user := models.User{UserId: "test-user"}

	// Run handler
	greader.handleParseFullArticle(w, req, user)

	// Check response status code
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.StatusCode)
	}

	// Verify that the savedParsedContent (and response content) has both sanitization and rewriting transformations applied:
	// 1. Sanitization: script tags must be stripped.
	if strings.Contains(savedParsedContent, "evil script") || strings.Contains(savedParsedContent, "<script>") {
		t.Errorf("saved parsed content was not sanitized (contained script): %s", savedParsedContent)
	}

	// 2. Link rewriting: "subpage/link" must be rewritten using the base article URL (ts.URL + "/folder/article.html").
	expectedLink := ts.URL + "/folder/subpage/link"
	if !strings.Contains(savedParsedContent, expectedLink) {
		t.Errorf("saved parsed content did not have link resolved (expected %q): %s", expectedLink, savedParsedContent)
	}

	// 3. Image rewriting: "images/photo.jpg" must be rewritten to absolute URL.
	expectedImg := ts.URL + "/folder/images/photo.jpg"
	if !strings.Contains(savedParsedContent, expectedImg) {
		t.Errorf("saved parsed content did not have image resolved (expected %q): %s", expectedImg, savedParsedContent)
	}
}

// editTagRequest builds an edit-tag request carrying the given hex article IDs.
func editTagRequest(tag string, addTag bool, hexIds ...string) *http.Request {
	form := url.Values{}
	form.Add("T", "post_token")
	if addTag {
		form.Add("a", tag)
	} else {
		form.Add("r", tag)
	}
	for _, id := range hexIds {
		form.Add("i", id)
	}
	req := httptest.NewRequest("POST", "/greader/reader/api/0/edit-tag", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// Clients batch hundreds of IDs into one request here, so the handler must
// issue a single bulk mark rather than one call per ID.
func TestHandleEditTagMarksAllIdsInOneCall(t *testing.T) {
	var calls int
	var gotIds []int64
	var gotMark models.MarkAction

	mockDB := &storage.MockDB{
		OnMarkArticlesForUser: func(_ models.User, ids []int64, mark models.MarkAction) (int64, error) {
			calls++
			gotIds = ids
			gotMark = mark
			return int64(len(ids)), nil
		},
	}

	greader := GReader{d: mockDB}
	w := httptest.NewRecorder()
	req := editTagRequest(readStreamId, true, "3039", "1", "10c2b4a0bbed8001")
	greader.handleEditTag(w, req, models.User{UserId: "test-user"})

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	if calls != 1 {
		t.Errorf("MarkArticlesForUser called %d times, want exactly 1", calls)
	}
	// Article IDs are hex on the wire.
	want := []int64{12345, 1, 1207726252529385473}
	if len(gotIds) != len(want) {
		t.Fatalf("marked %v, want %v", gotIds, want)
	}
	for i := range want {
		if gotIds[i] != want[i] {
			t.Errorf("marked[%d] = %d, want %d", i, gotIds[i], want[i])
		}
	}
	if gotMark != models.MarkActionRead {
		t.Errorf("mark = %v, want MarkActionRead", gotMark)
	}
}

// Removing the read tag is how clients mark unread, rather than adding the
// kept-unread tag.
func TestHandleEditTagRemoveReadMarksUnread(t *testing.T) {
	var gotMark models.MarkAction
	mockDB := &storage.MockDB{
		OnMarkArticlesForUser: func(_ models.User, ids []int64, mark models.MarkAction) (int64, error) {
			gotMark = mark
			return int64(len(ids)), nil
		},
	}

	greader := GReader{d: mockDB}
	w := httptest.NewRecorder()
	greader.handleEditTag(w, editTagRequest(readStreamId, false, "3039"), models.User{UserId: "u"})

	if gotMark != models.MarkActionUnread {
		t.Errorf("mark = %v, want MarkActionUnread", gotMark)
	}
}

// A malformed article ID is the client's fault, not the server's.
func TestHandleEditTagRejectsUnparseableIdAsBadRequest(t *testing.T) {
	greader := GReader{d: &storage.MockDB{}}
	w := httptest.NewRecorder()
	greader.handleEditTag(w, editTagRequest(readStreamId, true, "nothex"), models.User{UserId: "u"})

	if got := w.Result().StatusCode; got != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", got, http.StatusBadRequest)
	}
}

func TestMarkAllAsReadRejectsUnparseableIdAsBadRequest(t *testing.T) {
	for _, tc := range []struct{ name, key string }{
		{"folder", "t"},
		{"feed", "s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			form := url.Values{}
			form.Add("T", "post_token")
			form.Add(tc.key, "not-a-number")
			req := httptest.NewRequest("POST", "/greader/reader/api/0/mark-all-as-read", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			w := httptest.NewRecorder()
			GReader{d: &storage.MockDB{}}.markAllAsRead(w, req, models.User{UserId: "u"})

			if got := w.Result().StatusCode; got != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", got, http.StatusBadRequest)
			}
		})
	}
}

// Clients use this header to distinguish "refetch your post token" from a real
// auth failure, so it has to survive to the wire.
func TestReturnInvalidPostTokenSendsHeader(t *testing.T) {
	w := httptest.NewRecorder()
	GReader{}.returnInvalidPostToken(w, "stale")

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if got := resp.Header.Get(invalidPostTokenHeader); got != "true" {
		t.Errorf("%s = %q, want %q", invalidPostTokenHeader, got, "true")
	}
}

// The auth token neither expires nor can be revoked short of a password
// change, so it must never reach the log.
func TestDumpRequestRedactedMasksCredentials(t *testing.T) {
	const secret = "GoogleLogin auth=super-secret-token"

	req := httptest.NewRequest("POST", "/greader/reader/api/0/subscription/edit", nil)
	req.Header.Set("Authorization", secret)
	req.Header.Set("Cookie", "goliath_token=also-secret")
	req.Header.Set("User-Agent", "TestClient/1.0")

	dump, err := dumpRequestRedacted(req)
	if err != nil {
		t.Fatalf("dumpRequestRedacted: %v", err)
	}

	got := string(dump)
	for _, leaked := range []string{"super-secret-token", "also-secret"} {
		if strings.Contains(got, leaked) {
			t.Errorf("dump leaked %q:\n%s", leaked, got)
		}
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("dump missing redaction marker:\n%s", got)
	}
	// Diagnostics that are not credentials must survive.
	if !strings.Contains(got, "TestClient/1.0") {
		t.Errorf("dump dropped User-Agent:\n%s", got)
	}
	// The caller's request must be left exactly as it was found.
	if got := req.Header.Get("Authorization"); got != secret {
		t.Errorf("Authorization header not restored: %q", got)
	}
}

// A request whose form cannot be parsed must not reach a handler: the error
// status has already been written, and dispatching anyway writes a second one.
func TestRouteDoesNotDispatchWhenFormParsingFails(t *testing.T) {
	req := httptest.NewRequest("POST", "/greader/reader/api/0/token", nil)
	req.URL.RawQuery = "%zz"

	w := httptest.NewRecorder()
	GReader{d: &storage.MockDB{}}.route(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	// route sets this only after preprocessing succeeds, so its absence shows
	// dispatch was skipped rather than merely erroring later.
	if got := resp.Header.Get("Content-Type"); got == "application/json" {
		t.Error("route continued past a failed form parse")
	}
}
