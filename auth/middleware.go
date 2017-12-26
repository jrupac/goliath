package auth

import (
	"fmt"
	"github.com/jrupac/goliath/storage"
	"net/http"
)

const (
	authCookie = "goliath"
	loginPath  = "/login"
)

// Middleware is a wrapper around a http.Handler that contains a pointer to the database connection.
type Middleware struct {
	wrapped http.Handler
	d       *storage.Database
	root    string
}

// WithAuth returns a Middleware that checks authentication before forwarding requests to the given handler.
func WithAuth(h http.Handler, d *storage.Database, root string) Middleware {
	return Middleware{h, d, root}
}

// ServeHTTP implements the http.Handler interface and checks authentication on each request.
func (m Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Special-case going to login page without cookies.
	if r.URL.Path == loginPath {
		// Further routing is handled client-side.
		http.ServeFile(w, r, fmt.Sprintf("%s/index.html", m.root))
		return
	}

	if VerifyCookie(m.d, r) {
		m.wrapped.ServeHTTP(w, r)
		return
	}

	returnRedirect(w, r)
	return
}

// VerifyCookie checks a request for an auth cookie and authenticates it against the database.
func VerifyCookie(d *storage.Database, r *http.Request) bool {
	cookie, err := r.Cookie(authCookie)
	// Only ErrNoCookie can be returned here, which just means that the specified
	// cookie doesn't exist.
	if err != nil {
		return false
	}

	_, err = d.GetUserByKey(cookie.Value)
	return err == nil
}

func returnRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, loginPath, 302)
}

func returnError(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
}

func returnSuccess(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
