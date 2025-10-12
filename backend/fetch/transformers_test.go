package fetch

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/rss"
)

func TestPrependMediaToHtml(t *testing.T) {
	testCases := []struct {
		name        string
		imgSrc      string
		htmlContent string
		expected    string
	}{
		{
			name:        "prepend to empty string",
			imgSrc:      "http://example.com/image.png",
			htmlContent: "",
			expected:    `<html><head></head><body><img src="http://example.com/image.png"/></body></html>`,
		},
		{
			name:        "prepend to html fragment",
			imgSrc:      "http://example.com/image.png",
			htmlContent: "<p>Hello</p>",
			expected:    `<html><head></head><body><img src="http://example.com/image.png"/><p>Hello</p></body></html>`,
		},
		{
			name:        "prepend to full html document",
			imgSrc:      "http://example.com/image.png",
			htmlContent: `<html><head><title>Title</title></head><body><p>Hello</p></body></html>`,
			expected:    `<html><head><title>Title</title></head><body><img src="http://example.com/image.png"/><p>Hello</p></body></html>`,
		},
		{
			name:        "image source needs escaping",
			imgSrc:      "http://example.com/image.png?a=1&b=2\"",
			htmlContent: "<p>Hello</p>",
			expected:    `<html><head></head><body><img src="http://example.com/image.png?a=1&amp;b=2&#34;"/><p>Hello</p></body></html>`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := prependMediaToHtml(tc.imgSrc, tc.htmlContent)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestProcessItem(t *testing.T) {
	feed := &models.Feed{
		ID:       1,
		FolderID: 1,
		Title:    "Test Feed",
	}

	baseItem := func() *rss.Item {
		return &rss.Item{
			Title:     "Test Title",
			Link:      "http://example.com/article",
			Content:   "<p>Some content.</p>",
			Summary:   "<p>Some summary.</p>",
			Date:      time.Now(),
			DateValid: true,
		}
	}

	t.Run("with image enclosure", func(t *testing.T) {
		item := baseItem()
		enclosureURL := "http://example.com/image.jpg"
		item.Enclosures = []*rss.Enclosure{
			{
				URL:  enclosureURL,
				Type: "image/jpeg",
			},
		}
		originalContent := item.Content

		article := processItem(feed, item)

		if !strings.Contains(article.Content, enclosureURL) {
			t.Errorf("article content should contain enclosure URL %q, but was %q", enclosureURL, article.Content)
		}

		if article.Content == originalContent {
			t.Error("article content should be transformed, but was same as original")
		}

		// Also check Parsed field
		if !strings.Contains(article.Parsed, enclosureURL) {
			t.Errorf("article parsed content should contain enclosure URL %q, but was %q", enclosureURL, article.Parsed)
		}
	})

	t.Run("with non-image enclosure", func(t *testing.T) {
		item := baseItem()
		enclosureURL := "http://example.com/audio.mp3"
		item.Enclosures = []*rss.Enclosure{
			{
				URL:  enclosureURL,
				Type: "audio/mpeg",
			},
		}

		article := processItem(feed, item)

		if strings.Contains(article.Content, enclosureURL) {
			t.Errorf("article content should not contain non-image enclosure URL %q, but was %q", enclosureURL, article.Content)
		}
	})

	t.Run("with empty title", func(t *testing.T) {
		item := baseItem()
		item.Title = ""
		article := processItem(feed, item)
		if article.Title != "(Untitled)" {
			t.Errorf("expected title to be '(Untitled)', but got %q", article.Title)
		}
	})

	t.Run("with no date", func(t *testing.T) {
		item := baseItem()
		item.Date = time.Time{}
		item.DateValid = false
		article := processItem(feed, item)
		if !article.SyntheticDate {
			t.Error("expected SyntheticDate to be true")
		}
	})

	t.Run("with content in summary", func(t *testing.T) {
		item := baseItem()
		item.Content = ""
		article := processItem(feed, item)
		if !strings.Contains(article.Content, "Some summary.") {
			t.Errorf("expected content to be taken from summary, but got %q", article.Content)
		}
	})

	t.Run("sanitizes script tags", func(t *testing.T) {
		oldSanitizeHTML := *sanitizeHTML
		*sanitizeHTML = true
		defer func() { *sanitizeHTML = oldSanitizeHTML }()

		item := baseItem()
		item.Content = "<p>Safe content</p><script>alert('pwned')</script>"

		article := processItem(feed, item)

		if strings.Contains(article.Content, "<script>") {
			t.Errorf("article content should not contain script tags, but was %q", article.Content)
		}
		if strings.Contains(article.Content, "pwned") {
			t.Errorf("article content should not contain script content, but was %q", article.Content)
		}
		if !strings.Contains(article.Content, "Safe content") {
			t.Errorf("article content should still contain safe content, but was %q", article.Content)
		}
	})
}

func TestMaybeUnescapeHtml(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no escaping",
			input:    "this is a test string",
			expected: "this is a test string",
		},
		{
			name:     "one escape sequence",
			input:    "this is a &amp; test string",
			expected: "this is a &amp; test string",
		},
		{
			name:     "multiple escape sequences",
			input:    "this is a &amp; test &lt; string &gt;",
			expected: "this is a & test < string >",
		},
		{
			name:     "quotes and apos",
			input:    `&#34;hello&#34; &amp; &apos;world&apos;`,
			expected: `"hello" & 'world'`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := maybeUnescapeHtml(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestMaybeRewriteImageSourceUrls(t *testing.T) {
	// Save original flag values and restore them after the test.
	oldProxyInsecure := *proxyInsecureImages
	oldProxySecure := *proxySecureImages
	oldProxyUrlBase := *proxyUrlBase
	defer func() {
		*proxyInsecureImages = oldProxyInsecure
		*proxySecureImages = oldProxySecure
		*proxyUrlBase = oldProxyUrlBase
	}()

	*proxyInsecureImages = true
	*proxySecureImages = false
	*proxyUrlBase = "https://proxy.example.com"

	t.Run("rewrites insecure http image", func(t *testing.T) {
		html := `<img src="http://example.com/insecure.jpg">`
		rewritten := maybeRewriteImageSourceUrls(html)
		expectedUrl := "https://proxy.example.com/cache?url=http%3A%2F%2Fexample.com%2Finsecure.jpg"
		expectedHtml := fmt.Sprintf(`<html><head></head><body><img src="%s"/></body></html>`, expectedUrl)
		if rewritten != expectedHtml {
			t.Errorf("expected %q, got %q", expectedHtml, rewritten)
		}
	})

	t.Run("does not rewrite secure https image by default", func(t *testing.T) {
		html := `<html><head></head><body><img src="https://example.com/secure.jpg"/></body></html>`
		rewritten := maybeRewriteImageSourceUrls(html)
		if rewritten != html {
			t.Errorf("expected html to be unchanged, but it was rewritten to %q", rewritten)
		}
	})

	t.Run("rewrites secure https image when configured", func(t *testing.T) {
		*proxySecureImages = true
		defer func() { *proxySecureImages = false }()
		html := `<img src="https://example.com/secure.jpg">`
		rewritten := maybeRewriteImageSourceUrls(html)
		expectedUrl := "https://proxy.example.com/cache?url=https%3A%2F%2Fexample.com%2Fsecure.jpg"
		expectedHtml := fmt.Sprintf(`<html><head></head><body><img src="%s"/></body></html>`, expectedUrl)
		if rewritten != expectedHtml {
			t.Errorf("expected %q, got %q", expectedHtml, rewritten)
		}
	})

	t.Run("does not rewrite if proxying is disabled", func(t *testing.T) {
		*proxyInsecureImages = false
		defer func() { *proxyInsecureImages = true }()
		html := `<html><head></head><body><img src="http://example.com/insecure.jpg"/></body></html>`
		rewritten := maybeRewriteImageSourceUrls(html)
		if rewritten != html {
			t.Errorf("expected html to be unchanged, but it was rewritten to %q", rewritten)
		}
	})
}

func TestExtractTextFromHtmlUnsafe(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple html",
			input:    "<p>Hello, <b>world</b>!</p>",
			expected: "Hello, world!",
		},
		{
			name:     "with script tags",
			input:    "<p>Some text</p><script>alert('pwned')</script>",
			expected: "Some textalert('pwned')",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "plain text",
			input:    "just text",
			expected: "just text",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractTextFromHtmlUnsafe(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestStemWord(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"running", "running", "run"},
		{"jumps", "jumps", "jump"},
		{"happily", "happily", "happili"},
		{"punctuation", "awe.some!", "awesom"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := stemWord(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestMaybeMuteArticle(t *testing.T) {
	article := models.Article{
		FeedID:  1,
		Title:   "This is a test article",
		Summary: "Content about cats and dogs.",
		Content: "More content about programming.",
	}

	t.Run("mutes based on title", func(t *testing.T) {
		if !maybeMuteArticle(article, []string{"article"}, nil) {
			t.Error("expected to mute article based on title")
		}
	})

	t.Run("mutes based on summary", func(t *testing.T) {
		if !maybeMuteArticle(article, []string{"cats"}, nil) {
			t.Error("expected to mute article based on summary")
		}
	})

	t.Run("does not mute for non-matching words", func(t *testing.T) {
		if maybeMuteArticle(article, []string{"politics"}, nil) {
			t.Error("did not expect to mute article")
		}
	})

	t.Run("does not mute for unmuted feed", func(t *testing.T) {
		if maybeMuteArticle(article, []string{"article"}, []int64{1}) {
			t.Error("did not expect to mute article from unmuted feed")
		}
	})
}

func TestGetSimilarExistingArticles(t *testing.T) {
	oldStrictDedup := *strictDedup
	oldMaxEditDedup := *maxEditDedup
	defer func() {
		*strictDedup = oldStrictDedup
		*maxEditDedup = oldMaxEditDedup
	}()

	existingArticles := []models.Article{
		{ID: 1, Link: "http://example.com/a", Title: "Original Title", Summary: "Original Summary", Read: false},
		{ID: 2, Link: "http://example.com/b", Title: "Another Article", Summary: "Different Summary", Read: true},
		{ID: 3, Link: "http://example.com/a", Title: "Original Title!", Summary: "Original Summary", Read: true},
		{ID: 4, Link: "http://example.com/c", Title: "A New Article", Summary: "Completely different", Read: false},
	}
	newArticle := models.Article{Link: "http://example.com/a", Title: "Original Title", Summary: "Original Summary"}

	t.Run("fuzzy matching", func(t *testing.T) {
		*strictDedup = false
		*maxEditDedup = 0.3

		unreadIDs, readIDs := getSimilarExistingArticles(existingArticles, newArticle)

		if len(unreadIDs) != 1 || unreadIDs[0] != 1 {
			t.Errorf("expected unreadIDs [1], got %v", unreadIDs)
		}

		if len(readIDs) != 1 || readIDs[0] != 3 {
			t.Errorf("expected readIDs [3], got %v", readIDs)
		}
	})

	t.Run("strict matching", func(t *testing.T) {
		*strictDedup = true

		unreadIDs, readIDs := getSimilarExistingArticles(existingArticles, newArticle)

		if len(unreadIDs) != 1 || unreadIDs[0] != 1 {
			t.Errorf("expected unreadIDs [1] with strict dedup, got %v", unreadIDs)
		}

		if len(readIDs) != 1 || readIDs[0] != 3 {
			t.Errorf("expected readIDs [3] with strict dedup, got %v", readIDs)
		}
	})
}
