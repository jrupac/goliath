package cache

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// withKey installs a signing key for the duration of a test. Without one
// nothing can be signed, and every request is refused.
func withKey(t *testing.T) {
	t.Helper()
	original := *imageProxyKey
	*imageProxyKey = "test-image-proxy-key"
	t.Cleanup(func() { *imageProxyKey = original })
}

// proxyRequest builds a correctly signed request for a target.
func proxyRequest(target string) *http.Request {
	return httptest.NewRequest("GET", fmt.Sprintf("/cache?url=%s&%s=%s",
		url.QueryEscape(target), SignatureParam, url.QueryEscape(SignImageTarget(target))), nil)
}

// imageBackend serves one image, recording what the proxy asked it for.
func imageBackend(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

// The proxy is reachable from anywhere, so what stops it being an open proxy is
// that it will only fetch a URL this server signed.
func TestImageProxyRefusesUnsignedRequests(t *testing.T) {
	withKey(t)

	backend := imageBackend(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("an unsigned request reached the target")
	})

	for _, tc := range []struct {
		name string
		uri  string
	}{
		{"no signature", "/cache?url=" + url.QueryEscape(backend.URL)},
		{"wrong signature", "/cache?url=" + url.QueryEscape(backend.URL) + "&sig=bm90LWEtc2ln"},
		{
			"signature for a different target",
			"/cache?url=" + url.QueryEscape(backend.URL) + "&sig=" + url.QueryEscape(SignImageTarget("http://example.com/other.png")),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proxy := NewImageProxy()
			rr := httptest.NewRecorder()
			proxy.ServeHTTP(rr, httptest.NewRequest("GET", tc.uri, nil))

			if rr.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
			}
		})
	}
}

func TestImageProxyRequiresAUrl(t *testing.T) {
	withKey(t)

	rr := httptest.NewRecorder()
	NewImageProxy().ServeHTTP(rr, httptest.NewRequest("GET", "/cache", nil))

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// An image proxy that relays anything is a way of reading arbitrary documents
// through this server.
func TestImageProxyRefusesNonImageContent(t *testing.T) {
	withKey(t)

	backend := imageBackend(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "not an image")
	})

	proxy := NewImageProxy().(*imageProxy)
	proxy.Client = backend.Client()

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, proxyRequest(backend.URL))

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadGateway)
	}
}

// Nothing is cached here, so the browser is the only cache, and it can only
// decide well if it is told what the origin said.
func TestImageProxyForwardsCachingHeaders(t *testing.T) {
	withKey(t)

	backend := imageBackend(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "max-age=99999, public")
		w.Header().Set("ETag", `"abc123"`)
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")
		_, _ = fmt.Fprint(w, "png-bytes")
	})

	proxy := NewImageProxy().(*imageProxy)
	proxy.Client = backend.Client()

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, proxyRequest(backend.URL))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	for header, want := range map[string]string{
		"Content-Type":  "image/png",
		"Cache-Control": "max-age=99999, public",
		"ETag":          `"abc123"`,
		"Last-Modified": "Wed, 21 Oct 2015 07:28:00 GMT",
	} {
		if got := rr.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	if body, _ := io.ReadAll(rr.Body); string(body) != "png-bytes" {
		t.Errorf("body = %q, want %q", body, "png-bytes")
	}
}

// A browser holding a cached copy should be able to revalidate rather than
// refetch, which only works if its conditional request reaches the origin.
func TestImageProxyRelaysRevalidation(t *testing.T) {
	withKey(t)

	var gotConditional string
	backend := imageBackend(t, func(w http.ResponseWriter, r *http.Request) {
		gotConditional = r.Header.Get("If-None-Match")
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"abc123"`)
		w.WriteHeader(http.StatusNotModified)
	})

	proxy := NewImageProxy().(*imageProxy)
	proxy.Client = backend.Client()

	req := proxyRequest(backend.URL)
	req.Header.Set("If-None-Match", `"abc123"`)
	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	if gotConditional != `"abc123"` {
		t.Errorf("origin saw If-None-Match %q, want %q", gotConditional, `"abc123"`)
	}
	if rr.Code != http.StatusNotModified {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotModified)
	}
}

// The target is named by feed content, so the amount relayed has to be bounded
// by something other than the publisher's good intentions.
func TestImageProxyBoundsResponseSize(t *testing.T) {
	withKey(t)

	backend := imageBackend(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		// Longer than the bound, written in chunks so the test does not hold
		// the whole thing at once.
		chunk := strings.Repeat("x", 1<<20)
		for i := 0; i < (maxImageBytes>>20)+2; i++ {
			if _, err := io.WriteString(w, chunk); err != nil {
				return
			}
		}
	})

	proxy := NewImageProxy().(*imageProxy)
	proxy.Client = backend.Client()

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, proxyRequest(backend.URL))

	if got := rr.Body.Len(); got > maxImageBytes {
		t.Errorf("relayed %d bytes, want at most %d", got, maxImageBytes)
	}
}

func TestImageProxyReportsBadOriginStatus(t *testing.T) {
	withKey(t)

	backend := imageBackend(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	proxy := NewImageProxy().(*imageProxy)
	proxy.Client = backend.Client()

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, proxyRequest(backend.URL))

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadGateway)
	}
}

// Failing outright rather than redirecting: the redirect this replaces sent the
// browser to the image's insecure origin, which is what proxying exists to
// avoid, and did it without anything looking wrong.
func TestDenyUnauthenticatedDoesNotRedirect(t *testing.T) {
	rr := httptest.NewRecorder()
	DenyUnauthenticated(rr, httptest.NewRequest("GET", "/cache?url=http://example.com/a.png", nil))

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
	if location := rr.Header().Get("Location"); location != "" {
		t.Errorf("sent the browser to %q instead of refusing", location)
	}
}
