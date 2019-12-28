package auth

import (
	"fmt"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"net/http"
)

const (
	authCookie = "goliath"
	loginPath  = "/login"
)

// Redirector is the type of a custom HTTP handler to be called upon failed
// cookie verification.
type Redirector = func(http.ResponseWriter, *http.Request)

// Middleware is a wrapper around a http.Handler that contains a pointer to the database connection.
type Middleware struct {
	wrapped    http.Handler
	d          *storage.Database
	root       string
	redirector Redirector
}

// WithAuth returns a Middleware that checks authentication before forwarding requests to the given handler.
func WithAuth(h http.Handler, d *storage.Database, root string, redirector Redirector) Middleware {
	return Middleware{h, d, root, redirector}
}

// ServeHTTP implements the http.Handler interface and checks authentication on each request.
func (m Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Special-case going to login page without cookies.
	if r.URL.Path == loginPath {
		// Further routing is handled client-side.
		http.ServeFile(w, r, fmt.Sprintf("%s/index.html", m.root))
		return
	}

	if _, err := VerifyCookie(m.d, r); err == nil {
		m.wrapped.ServeHTTP(w, r)
		return
	}

	if m.redirector != nil {
		m.redirector(w, r)
	} else {
		returnRedirect(w, r)
	}
	return
}

// VerifyCookie checks a request for an auth cookie and authenticates it against the database.
func VerifyCookie(d *storage.Database, r *http.Request) (models.User, error) {
	cookie, err := r.Cookie(authCookie)
	// Only ErrNoCookie can be returned here, which just means that the specified
	// cookie doesn't exist.
	if err != nil {
		return models.User{}, err
	}

	return d.GetUserByKey(cookie.Value)
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
