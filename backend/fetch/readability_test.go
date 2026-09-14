package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jrupac/goliath/utils"
)

// An article's link comes from its feed, so extraction is guarded like a feed
// fetch. The refusal happens in the dialer, so nothing is sent.
func TestExtractionRefusesInternalAddresses(t *testing.T) {
	e := newArticleExtractor(time.Second, "Test-Agent", nil)

	for _, target := range []string{
		"http://127.0.0.1/foo",
		"http://169.254.169.254/latest/meta-data/",
		"http://100.64.0.1/",
	} {
		if _, err := e.Extract(context.Background(), target); !errors.Is(err, utils.ErrBlockedAddress) {
			t.Errorf("%s: err = %v, want %v", target, err, utils.ErrBlockedAddress)
		}
	}
}

const articleHTML = `
<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>Test Title</title>
</head>
<body>
	<article>
		<h1>Interesting Article Title</h1>
		<p>This is the main article body content that should be extracted by readability.</p>
	</article>
	<aside>This is sidebar content that should be filtered out.</aside>
</body>
</html>`

func TestExtractionSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(articleHTML))
	}))
	defer ts.Close()

	// Bypass dial block in tests by using ts.Client()
	e := &articleExtractor{
		client:    ts.Client(),
		userAgent: "Test-Agent",
	}

	parsed, err := e.Extract(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("unexpected extraction error: %v", err)
	}

	if !strings.Contains(parsed, "Interesting Article Title") {
		t.Errorf("expected output to contain title, got: %s", parsed)
	}
	if !strings.Contains(parsed, "This is the main article body content") {
		t.Errorf("expected output to contain body content, got: %s", parsed)
	}
	if strings.Contains(parsed, "sidebar content") {
		t.Errorf("expected sidebar content to be stripped out, got: %s", parsed)
	}
}

// Articles move between hosts, so a redirect to another one is followed; each
// hop is dialed through the guard, so a redirect inward is still refused.
func TestExtractionFollowsRedirectsThroughTheGuard(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(articleHTML))
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/moved":
			http.Redirect(w, r, target.URL+"/article", http.StatusFound)
		case "/inward":
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
		}
	}))
	defer origin.Close()

	// The test servers are on loopback, so they are reached as a configured
	// bridge would be. The redirect inward is not on the list.
	allowed, err := utils.NewAddressAllowlist([]string{
		strings.TrimPrefix(origin.URL, "http://"),
		strings.TrimPrefix(target.URL, "http://"),
	})
	if err != nil {
		t.Fatalf("NewAddressAllowlist: %v", err)
	}
	e := newArticleExtractor(5*time.Second, "Test-Agent", allowed)

	content, err := e.Extract(context.Background(), origin.URL+"/moved")
	if err != nil {
		t.Fatalf("redirect to another host: %v", err)
	}
	if !strings.Contains(content, "Interesting Article Title") {
		t.Errorf("extracted %q, want the target's article", content)
	}

	if _, err := e.Extract(context.Background(), origin.URL+"/inward"); !errors.Is(err, utils.ErrBlockedAddress) {
		t.Errorf("redirect inward: err = %v, want %v", err, utils.ErrBlockedAddress)
	}
}

func TestPromoteImageSources(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "data-src promotion",
			input:    `<img src="placeholder.jpg" data-src="highres.jpg" />`,
			expected: `src="highres.jpg"`,
		},
		{
			name:     "data-loading JSON promotion",
			input:    `<img src="placeholder.jpg" data-loading='{"mobile":"mob.jpg","desktop":"desk.jpg"}' />`,
			expected: `src="desk.jpg"`,
		},
		{
			name:     "srcset promotion",
			input:    `<img src="placeholder.jpg" srcset="low.jpg 100w, high.jpg 500w" />`,
			expected: `src="high.jpg"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := promoteImageSources(tc.input)
			if !strings.Contains(result, tc.expected) {
				t.Errorf("expected result to contain %q, but got %q", tc.expected, result)
			}
		})
	}
}

func TestRewriteFragmentUrls(t *testing.T) {
	articleURL, _ := url.Parse("https://example.com/post")

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "rewrites absolute match with fragment",
			input:    `<a href="https://example.com/post#fn-1">Footnote 1</a>`,
			expected: `<a href="#fn-1">Footnote 1</a>`,
		},
		{
			name:     "rewrites absolute match with trailing slash and fragment",
			input:    `<a href="https://example.com/post/#fn-1">Footnote 1</a>`,
			expected: `<a href="#fn-1">Footnote 1</a>`,
		},
		{
			name:     "ignores absolute match without fragment",
			input:    `<a href="https://example.com/post">Post Link</a>`,
			expected: `href="https://example.com/post"`,
		},
		{
			name:     "ignores different host link with fragment",
			input:    `<a href="https://other.com/post#fn-1">Other Link</a>`,
			expected: `href="https://other.com/post#fn-1"`,
		},
		{
			name:     "ignores different path link with fragment",
			input:    `<a href="https://example.com/about#team">Team Section</a>`,
			expected: `href="https://example.com/about#team"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := rewriteFragmentUrls(tc.input, articleURL)
			if !strings.Contains(result, tc.expected) {
				t.Errorf("expected result to contain %q, but got %q", tc.expected, result)
			}
		})
	}
}
