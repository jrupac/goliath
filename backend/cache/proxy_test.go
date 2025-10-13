package cache

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestImageProxy_ServeHTTP(t *testing.T) {
	t.Run("no url parameter", func(t *testing.T) {
		proxy := NewImageProxy()
		req := httptest.NewRequest("GET", "/cache", nil)
		rr := httptest.NewRecorder()

		proxy.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("proxy success", func(t *testing.T) {
		// Create a mock backend server
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "proxied content")
		}))
		defer backend.Close()

		proxy := NewImageProxy().(*imageProxy)
		proxy.Client = backend.Client()

		reqUrl := fmt.Sprintf("/cache?url=%s", url.QueryEscape(backend.URL))
		req := httptest.NewRequest("GET", reqUrl, nil)
		rr := httptest.NewRecorder()

		proxy.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		body, _ := io.ReadAll(rr.Body)
		if string(body) != "proxied content" {
			t.Errorf("expected body %q, got %q", "proxied content", string(body))
		}
	})

	t.Run("proxy backend fails", func(t *testing.T) {
		// Create a mock backend server that always fails
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer backend.Close()

		proxy := NewImageProxy().(*imageProxy)
		proxy.Client = backend.Client()

		reqUrl := fmt.Sprintf("/cache?url=%s", url.QueryEscape(backend.URL))
		req := httptest.NewRequest("GET", reqUrl, nil)
		rr := httptest.NewRecorder()

		proxy.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadGateway {
			t.Errorf("expected status %d, got %d", http.StatusBadGateway, rr.Code)
		}
	})
}

func TestAuthErrorRedirect(t *testing.T) {
	t.Run("no url parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/auth-error", nil)
		rr := httptest.NewRecorder()

		AuthErrorRedirect(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("redirects successfully", func(t *testing.T) {
		targetUrl := "http://example.com/image.jpg"
		reqUrl := fmt.Sprintf("/auth-error?url=%s", url.QueryEscape(targetUrl))
		req := httptest.NewRequest("GET", reqUrl, nil)
		rr := httptest.NewRecorder()

		AuthErrorRedirect(rr, req)

		if rr.Code != http.StatusMovedPermanently {
			t.Errorf("expected status %d, got %d", http.StatusMovedPermanently, rr.Code)
		}

		location := rr.Header().Get("Location")
		if location != targetUrl {
			t.Errorf("expected redirect location %q, got %q", targetUrl, location)
		}
	})
}
