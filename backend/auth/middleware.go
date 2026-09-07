package auth

import (
	"fmt"
	"net/http"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
)

const loginPath = "/login"

// Redirector is an HTTP handler to be called upon failed cookie verification.
type Redirector = func(http.ResponseWriter, *http.Request)

// Middleware is a wrapper around a http.Handler that contains a pointer to the database connection.
type Middleware struct {
	wrapped      http.Handler
	d            storage.Database
	root         string
	redirector   Redirector
	verifyCookie bool
}

// WithAuth returns a Middleware that checks authentication before forwarding requests to the given handler.
func WithAuth(h http.Handler, d storage.Database, root string, redirector Redirector, verifyCookie bool) Middleware {
	return Middleware{h, d, root, redirector, verifyCookie}
}

// ServeHTTP implements the http.Handler interface and checks authentication on each request.
func (m Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Special-case going to login page when directed explicitly.
	if r.URL.Path == loginPath {
		// Further routing is handled client-side.
		http.ServeFile(w, r, fmt.Sprintf("%s/index.html", m.root))
		return
	}

	if m.verifyCookie {
		if _, err := VerifyCookie(m.d, r); err == nil {
			m.wrapped.ServeHTTP(w, r)
			return
		}
	} else {
		log.Infof("Serving without cookie verification: %s", r.URL.Path)
		m.wrapped.ServeHTTP(w, r)
		return
	}

	if m.redirector != nil {
		m.redirector(w, r)
	} else {
		returnRedirect(w, r)
	}
}

// VerifyCookie identifies the user behind a request's cookies.
func VerifyCookie(d storage.Database, r *http.Request) (models.User, error) {
	if token, ok := sessionFromCookie(r); ok {
		user, _, err := d.LookupSession(token)
		return user, err
	}

	// A browser that signed in before sessions existed still holds the old
	// cookie, and nothing prompts it to sign in again until this stops being
	// accepted. Removed once the deprecation window closes.
	cookie, err := r.Cookie(legacyAuthCookie)
	if err != nil {
		// Only ErrNoCookie can be returned here, which just means the cookie
		// is not present.
		return models.User{}, err
	}
	log.Warningf("Accepted legacy auth cookie")
	return d.GetUserByKey(models.Secret(cookie.Value))
}

func returnRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, loginPath, 302)
}

func returnLoginFailed(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
}

func returnError(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
}

func returnSuccess(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
