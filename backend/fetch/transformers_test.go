package fetch

import (
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
		Link:     "http://example.com/feed",
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
		enclosureURL := "/image.jpg"
		item.Enclosures = []*rss.Enclosure{
			{
				URL:  enclosureURL,
				Type: "image/jpeg",
			},
		}
		originalContent := item.Content

		article := processItem(feed, item)

		expectedUrl := "http://example.com/image.jpg"
		if !strings.Contains(article.Content, expectedUrl) {
			t.Errorf("article content should contain absolute enclosure URL %q, but was %q", expectedUrl, article.Content)
		}

		if article.Content == originalContent {
			t.Error("article content should be transformed, but was same as original")
		}

		// Also check Parsed field
		if !strings.Contains(article.Parsed, expectedUrl) {
			t.Errorf("article parsed content should contain absolute enclosure URL %q, but was %q", expectedUrl, article.Parsed)
		}
	})

	t.Run("with non-image enclosure", func(t *testing.T) {
		item := baseItem()
		enclosureURL := "/audio.mp3"
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

	t.Run("with relative link", func(t *testing.T) {
		item := baseItem()
		item.Link = "/relative/link"

		article := processItem(feed, item)

		if !strings.HasPrefix(article.Link, "http") {
			t.Errorf("article link should be absolute, but was %q", article.Link)
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

func TestProcessImageUrl(t *testing.T) {
	feedLink := "http://example.com/feed"

	t.Run("makes relative url absolute", func(t *testing.T) {
		imageUrl := "/foo.jpg"
		expected := "http://example.com/foo.jpg"
		result := processImageUrl(feedLink, imageUrl)
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})

	t.Run("handles absolute url", func(t *testing.T) {
		imageUrl := "https://othersite.com/foo.jpg"
		expected := "https://othersite.com/foo.jpg"
		result := processImageUrl(feedLink, imageUrl)
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})

	t.Run("proxies insecure image", func(t *testing.T) {
		oldProxyInsecure := *proxyInsecureImages
		*proxyInsecureImages = true
		defer func() { *proxyInsecureImages = oldProxyInsecure }()

		oldProxyUrlBase := *proxyUrlBase
		*proxyUrlBase = "https://proxy.example.com"
		defer func() { *proxyUrlBase = oldProxyUrlBase }()

		imageUrl := "http://insecure.com/foo.jpg"
		expected := "https://proxy.example.com/cache?url=http%3A%2F%2Finsecure.com%2Ffoo.jpg"
		result := processImageUrl(feedLink, imageUrl)
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})
}

func TestMaybeRewriteUrls(t *testing.T) {
	feed := &models.Feed{
		ID:       1,
		FolderID: 1,
		Title:    "Test Feed",
		Link:     "http://example.com/feed",
	}

	t.Run("rewrite relative url in <a> tag", func(t *testing.T) {
		content := "<a href='/relative'>Relative Link</a>"
		expected := "<html><head></head><body><a href=\"http://example.com/relative\">Relative Link</a></body></html>"
		result := maybeRewriteUrls(feed, content)
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})

	t.Run("proxies insecure image", func(t *testing.T) {
		oldProxyInsecure := *proxyInsecureImages
		*proxyInsecureImages = true
		defer func() { *proxyInsecureImages = oldProxyInsecure }()

		content := "Hello world <img src='http://insecure.com/foo.jpg'>"
		expected := "<html><head></head><body>Hello world <img src=\"/cache?url=http%3A%2F%2Finsecure.com%2Ffoo.jpg\"/></body></html>"
		result := maybeRewriteUrls(feed, content)
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})

	t.Run("rewrites relative URL and proxies insecure image", func(t *testing.T) {
		oldProxyInsecure := *proxyInsecureImages
		*proxyInsecureImages = true
		defer func() { *proxyInsecureImages = oldProxyInsecure }()

		content := "Hello world <img src='/foo.jpg'>"
		expected := "<html><head></head><body>Hello world <img src=\"/cache?url=http%3A%2F%2Fexample.com%2Ffoo.jpg\"/></body></html>"
		result := maybeRewriteUrls(feed, content)
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
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
