package auth

import (
	"net/http"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
)

const (
	// sessionCookie carries a session token. It is set by the server and never
	// read by the page: a script that can read it can walk away with a
	// credential good for as long as the session lives, whereas one that can
	// only send it is confined to the browser it is running in.
	sessionCookie = "goliath_session"

	// legacyAuthCookie carried the key derived from the username and password,
	// which is the same value the Fever API accepts. It never expired and could
	// not be revoked. Still read so that a browser holding one keeps working
	// until it logs in again; nothing sets it any more.
	legacyAuthCookie = "goliath"
)

// setSessionCookie hands a session to the browser.
func setSessionCookie(w http.ResponseWriter, r *http.Request, token models.Secret) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie,
		// Revealed at the boundary: the cookie is how the credential reaches
		// the client, which is the point of issuing one.
		Value: token.Reveal(),
		Path:  "/",
		// Kept from scripts. This is what the page gives up in exchange for not
		// having to hold a long-lived credential itself.
		HttpOnly: true,
		// Lax rather than Strict so that following a link into the reader still
		// arrives logged in. Cross-site form posts are what matters here, and
		// Lax blocks those; post tokens cover the rest.
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
		// Matches how long the session itself would survive unused, so the
		// cookie does not outlive what it refers to, and does not quietly
		// depend on the browser restoring session cookies to persist at all.
		MaxAge: int(storage.SessionIdleWindow().Seconds()),
	})
}

// clearSessionCookie removes the session cookie from the browser. The
// attributes have to match the ones it was set with or the browser keeps it.
func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
		MaxAge:   -1,
	})
}

// isSecureRequest reports whether the request reached the server over TLS.
//
// The check includes the forwarded header because this commonly runs behind a
// reverse proxy that terminates TLS, where the request arriving here is plain
// HTTP even though the browser's connection is not. Marking the cookie Secure
// on a connection that is genuinely plain would make it undeliverable, which is
// why this is detected rather than configured on.
func isSecureRequest(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// SessionFromRequest returns the session token a request carries in its
// cookies, if any. Exported so that the API surfaces can accept a browser's
// session without duplicating knowledge of where it is kept.
func SessionFromRequest(r *http.Request) (models.Secret, bool) {
	return sessionFromCookie(r)
}

// sessionFromCookie returns the session token a request carries, if any.
func sessionFromCookie(r *http.Request) (models.Secret, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	return models.Secret(cookie.Value), true
}
