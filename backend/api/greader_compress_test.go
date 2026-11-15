package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/klauspost/compress/gzhttp"
	"golang.org/x/crypto/bcrypt"
)

// compressEverything compresses every response, however small, so that only a
// handler's own opt-out can keep one uncompressed.
func compressEverything(t *testing.T, h http.Handler) http.Handler {
	t.Helper()
	wrap, err := gzhttp.NewWrapper(gzhttp.MinSize(0))
	if err != nil {
		t.Fatalf("NewWrapper: %v", err)
	}
	return wrap(h)
}

func TestResponsesCarryingCredentialsAreNeverCompressed(t *testing.T) {
	const password = "correct horse"
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	user := models.User{UserId: "user-1", Username: "someone", Key: "k", HashPass: models.Secret(hashed)}
	d := &storage.MockDB{
		OnGetUserByUsername: func(string) (models.User, error) { return user, nil },
		OnCreateSession: func(models.User, models.AuthScheme, string) (models.Secret, error) {
			return "gol1_issued", nil
		},
		OnLookupSession: func(models.Secret) (models.User, models.Session, error) {
			return user, models.Session{SessionId: "s", UserId: user.UserId, Scheme: models.AuthSchemeGReader}, nil
		},
	}
	h := compressEverything(t, GReader{d: d}.Handler())

	login := httptest.NewRequest("POST", "/greader/accounts/ClientLogin",
		strings.NewReader(url.Values{"Email": {"someone"}, "Passwd": {password}}.Encode()))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	token := httptest.NewRequest("GET", "/greader/reader/api/0/token", nil)
	token.Header.Set("Authorization", "GoogleLogin auth=gol1_issued")

	for name, r := range map[string]*http.Request{"ClientLogin": login, "token": token} {
		t.Run(name, func(t *testing.T) {
			r.Header.Set("Accept-Encoding", "gzip, zstd")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			if ce := w.Header().Get("Content-Encoding"); ce != "" {
				t.Errorf("Content-Encoding = %q, want none", ce)
			}
			if w.Header().Get(gzhttp.HeaderNoCompression) != "" {
				t.Error("the opt-out header reached the client")
			}
			if w.Body.Len() == 0 {
				t.Error("empty body")
			}
		})
	}
}

// Everything else is left to the server-wide policy, which compresses.
func TestOtherAPIResponsesMayBeCompressed(t *testing.T) {
	user := models.User{UserId: "user-1", Username: "someone", Key: "k"}
	d := &storage.MockDB{
		OnLookupSession: func(models.Secret) (models.User, models.Session, error) {
			return user, models.Session{SessionId: "s", UserId: user.UserId, Scheme: models.AuthSchemeGReader}, nil
		},
	}
	r := authorizedRequest("gol1_issued")
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	compressEverything(t, GReader{d: d}.Handler()).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ce := w.Header().Get("Content-Encoding"); ce != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", ce)
	}
}
