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
