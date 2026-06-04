package fetch

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip   net.IP
		want bool
	}{
		{net.ParseIP("127.0.0.1"), true},
		{net.ParseIP("::1"), true},
		{net.ParseIP("10.0.0.1"), true},
		{net.ParseIP("172.16.0.1"), true},
		{net.ParseIP("192.168.1.1"), true},
		{net.ParseIP("100.64.0.1"), true},
		{net.ParseIP("100.127.255.255"), true},
		{net.ParseIP("100.128.0.1"), false},
		{net.ParseIP("8.8.8.8"), false},
		{net.ParseIP("1.1.1.1"), false},
		{nil, true},
	}

	for _, tt := range tests {
		got := isPrivateIP(tt.ip)
		if got != tt.want {
			t.Errorf("isPrivateIP(%v) = %v, want %v", tt.ip, got, tt.want)
		}
	}
}

func TestSSRFBlocking(t *testing.T) {
	e := newArticleExtractor(1*time.Second, "Test-Agent")

	// 127.0.0.1 is a loopback IP, which should be blocked by the dialer Control function.
	_, err := e.Extract(context.Background(), "http://127.0.0.1:9999/foo")
	if err == nil {
		t.Fatal("expected error when fetching private/loopback IP, got nil")
	}

	if !strings.Contains(err.Error(), "SSRF protection") && !strings.Contains(err.Error(), "access to private IP") {
		t.Errorf("expected SSRF protection error, got: %v", err)
	}
}

func TestExtractionSuccess(t *testing.T) {
	htmlContent := `
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

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlContent))
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

func TestCrossHostRedirectBlocking(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			// Redirect to a different host (localhost vs 127.0.0.1)
			http.Redirect(w, r, "http://localhost/target", http.StatusFound)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	dummy := newArticleExtractor(1*time.Second, "Test-Agent")

	e := &articleExtractor{
		client: &http.Client{
			Transport:     ts.Client().Transport,
			CheckRedirect: dummy.client.CheckRedirect,
		},
		userAgent: "Test-Agent",
	}

	_, err := e.Extract(context.Background(), ts.URL+"/redirect")
	if err == nil {
		t.Fatal("expected redirect to be blocked, but it succeeded")
	}

	if !strings.Contains(err.Error(), "SSRF protection") && !strings.Contains(err.Error(), "cross-host redirect") {
		t.Errorf("expected cross-host redirect error, got: %v", err)
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
