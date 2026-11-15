package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrupac/goliath/storage"
	"github.com/klauspost/compress/zstd"
)

const testIconHash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

// testServer serves the real mux over a public folder holding one script, and
// a database holding one favicon, each large enough to be worth compressing.
func testServer(t *testing.T) (http.Handler, []byte, []byte) {
	t.Helper()
	dir := t.TempDir()
	script := []byte(strings.Repeat("console.log('goliath');\n", 400))
	if err := os.MkdirAll(filepath.Join(dir, "static"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "static", "app.js"), script, 0o644); err != nil {
		t.Fatal(err)
	}
	old := *publicFolder
	*publicFolder = dir
	t.Cleanup(func() { *publicFolder = old })

	icon := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 4096)...)
	d := &storage.MockDB{
		OnGetFaviconByHash: func(string) (string, []byte, error) { return "image/png", icon, nil },
	}
	return newMux(d, nil), script, icon
}

func get(h http.Handler, path string, header http.Header) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", path, nil)
	for k, v := range header {
		r.Header[k] = v
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) []byte {
	t.Helper()
	var rd io.Reader
	switch ce := w.Header().Get("Content-Encoding"); ce {
	case "":
		return w.Body.Bytes()
	case "gzip":
		zr, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatalf("gzip: %v", err)
		}
		rd = zr
	case "zstd":
		zr, err := zstd.NewReader(w.Body)
		if err != nil {
			t.Fatalf("zstd: %v", err)
		}
		defer zr.Close()
		rd = zr
	default:
		t.Fatalf("unexpected Content-Encoding %q", ce)
	}
	b, err := io.ReadAll(rd)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	return b
}

func TestResponsesAreCompressedInTheEncodingTheClientPrefers(t *testing.T) {
	h, script, _ := testServer(t)
	for _, tc := range []struct {
		accept string
		want   string
	}{
		{"", ""},
		{"gzip", "gzip"},
		{"gzip, deflate, br", "gzip"},
		{"gzip, deflate, br, zstd", "zstd"},
	} {
		t.Run(tc.accept, func(t *testing.T) {
			w := get(h, "/static/app.js", http.Header{"Accept-Encoding": {tc.accept}})
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			if got := w.Header().Get("Content-Encoding"); got != tc.want {
				t.Errorf("Content-Encoding = %q, want %q", got, tc.want)
			}
			if !strings.Contains(w.Header().Get("Vary"), "Accept-Encoding") {
				t.Errorf("Vary = %q, want it to name Accept-Encoding", w.Header().Get("Vary"))
			}
			if got := decode(t, w); !bytes.Equal(got, script) {
				t.Errorf("decoded %d bytes, want the %d-byte script", len(got), len(script))
			}
		})
	}
}

// Raster images are already compressed, and an icon's validator has to keep
// meaning the bytes the client holds.
func TestImagesAreSentAsTheyAre(t *testing.T) {
	h, _, icon := testServer(t)
	path := "/greader/ext/favicon/" + testIconHash
	accept := http.Header{"Accept-Encoding": {"gzip, zstd"}}

	w := get(h, path, accept)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ce := w.Header().Get("Content-Encoding"); ce != "" {
		t.Errorf("Content-Encoding = %q, want none", ce)
	}
	if !bytes.Equal(w.Body.Bytes(), icon) {
		t.Errorf("body is %d bytes, want the %d-byte icon", w.Body.Len(), len(icon))
	}

	accept.Set("If-None-Match", w.Header().Get("ETag"))
	if w := get(h, path, accept); w.Code != http.StatusNotModified {
		t.Errorf("revalidation status = %d, want 304", w.Code)
	}
}

func TestCompressibleTypes(t *testing.T) {
	for ct, want := range map[string]bool{
		"application/json":               true,
		"text/html; charset=utf-8":       true,
		"text/javascript; charset=utf-8": true,
		"image/svg+xml":                  true,
		"image/png":                      false,
		"image/x-icon":                   false,
		"IMAGE/GIF":                      false,
		"application/zip":                false,
	} {
		if got := compressible(ct); got != want {
			t.Errorf("compressible(%q) = %v, want %v", ct, got, want)
		}
	}
}
