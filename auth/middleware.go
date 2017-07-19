package auth

import (
	"fmt"
	"github.com/jrupac/goliath/storage"
	"net/http"
)

const (
	AUTH_COOKIE = "goliath"
	LOGIN_PATH  = "/login"
)

type authMiddleware struct {
	wrapped http.Handler
	d       *storage.Database
	root    string
}

func WithAuth(h http.Handler, d *storage.Database, root string) authMiddleware {
	return authMiddleware{h, d, root}
}

func (m authMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Special-case going to login page without cookies.
	if r.URL.Path == LOGIN_PATH {
		// Further routing is handled client-side.
		http.ServeFile(w, r, fmt.Sprintf("%s/index.html", m.root))
		return
	}

	if VerifyCookie(m.d, r) {
		m.wrapped.ServeHTTP(w, r)
		return
	} else {
		returnRedirect(w, r)
		return
	}
}

func VerifyCookie(d *storage.Database, r *http.Request) bool {
	cookie, err := r.Cookie(AUTH_COOKIE)
	// Only ErrNoCookie can be returned here, which just means that the specified
	// cookie doesn't exist.
	if err != nil {
		return false
	}

	_, err = d.GetUserByKey(cookie.Value)
	return err == nil
}

func returnRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, LOGIN_PATH, 302)
}

func returnError(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
}

func returnSuccess(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
