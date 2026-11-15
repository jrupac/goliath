package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
)

const testFaviconHash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

var testFaviconBytes = []byte("\x89PNG not really")

func faviconDB(mime string) *storage.MockDB {
	return &storage.MockDB{
		OnGetFaviconByHash: func(hash string) (string, []byte, error) {
			if hash != testFaviconHash {
				return "", nil, sql.ErrNoRows
			}
			return mime, testFaviconBytes, nil
		},
	}
}

func serveFavicon(d storage.Database, method, path string, header http.Header) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	for k, v := range header {
		r.Header[k] = v
	}
	w := httptest.NewRecorder()
	GReader{d: d}.Handler()(w, r)
	return w
}

// Clients fetch an icon URL as a plain image, without the API's credentials,
// so the route must answer a request that carries none.
func TestFaviconIsServedWithoutCredentialsAndCachedForever(t *testing.T) {
	w := serveFavicon(faviconDB("image/png"), "GET", faviconPathPrefix+testFaviconHash, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), testFaviconBytes) {
		t.Errorf("body = %q, want the stored image", w.Body.Bytes())
	}
	for header, want := range map[string]string{
		"Content-Type":           "image/png",
		"Cache-Control":          "public, max-age=31536000, immutable",
		"ETag":                   `"` + testFaviconHash + `"`,
		"X-Content-Type-Options": "nosniff",
	} {
		if got := w.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestFaviconAnswersARevalidationWithNotModified(t *testing.T) {
	w := serveFavicon(faviconDB("image/png"), "GET", faviconPathPrefix+testFaviconHash,
		http.Header{"If-None-Match": {`"` + testFaviconHash + `"`}})

	if w.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("a 304 carried %d bytes", w.Body.Len())
	}
}

func TestFaviconRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mime   string
		method string
		path   string
		want   int
	}{
		{"unknown hash", "image/png", "GET", faviconPathPrefix + strings.Repeat("0", 64), http.StatusNotFound},
		{"not a hash", "image/png", "GET", faviconPathPrefix + "../token", http.StatusNotFound},
		{"uppercase hash", "image/png", "GET", faviconPathPrefix + strings.ToUpper(testFaviconHash), http.StatusNotFound},
		// SVG can carry script, which would run as this origin if the URL were
		// opened directly.
		{"svg", "image/svg+xml", "GET", faviconPathPrefix + testFaviconHash, http.StatusNotFound},
		{"post", "image/png", "POST", faviconPathPrefix + testFaviconHash, http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := serveFavicon(faviconDB(tc.mime), tc.method, tc.path, nil)
			if w.Code != tc.want {
				t.Errorf("status = %d, want %d", w.Code, tc.want)
			}
			if w.Body.Len() != 0 {
				t.Errorf("a refusal carried %d bytes", w.Body.Len())
			}
		})
	}
}

func subscriptionIcons(t *testing.T, r *http.Request) map[string]string {
	t.Helper()
	d := &storage.MockDB{
		OnGetAllFeedsForUser: func(models.User) ([]models.Feed, error) {
			return []models.Feed{{ID: 7, FolderID: 3, Title: "With"}, {ID: 8, FolderID: 3, Title: "Without"}}, nil
		},
		OnGetFaviconHashesForUser: func(models.User) (map[models.FeedId]string, error) {
			return map[models.FeedId]string{7: testFaviconHash}, nil
		},
	}
	w := httptest.NewRecorder()
	GReader{d: d}.handleSubscriptionList(w, r, models.User{UserId: "u"})

	var res greaderSubscriptionList
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	icons := map[string]string{}
	for _, s := range res.Subscriptions {
		icons[s.Id] = s.IconUrl
	}
	return icons
}

// An icon URL is fetched later and from wherever the client is, so it has to
// name the address the client reached, not the one a proxy forwarded to.
func TestSubscriptionListLinksFaviconsAtTheAddressTheClientUsed(t *testing.T) {
	r := httptest.NewRequest("GET", "http://127.0.0.1:9999/greader/reader/api/0/subscription/list", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "goliath.example.com")

	icons := subscriptionIcons(t, r)

	if got, want := icons["feed/7"], "https://goliath.example.com"+faviconPathPrefix+testFaviconHash; got != want {
		t.Errorf("iconUrl = %q, want %q", got, want)
	}
	if got := icons["feed/8"]; got != "" {
		t.Errorf("iconUrl for a feed with no favicon = %q, want empty", got)
	}
}

func TestSubscriptionListLinksFaviconsAtTheRequestedHostWithoutAProxy(t *testing.T) {
	r := httptest.NewRequest("GET", "http://localhost:9999/greader/reader/api/0/subscription/list", nil)

	icons := subscriptionIcons(t, r)

	if got, want := icons["feed/7"], "http://localhost:9999"+faviconPathPrefix+testFaviconHash; got != want {
		t.Errorf("iconUrl = %q, want %q", got, want)
	}
}
