package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"golang.org/x/crypto/bcrypt"
)

type auth struct {
	Username string        `json:"username"`
	Password models.Secret `json:"password"`
}

func (a *auth) valid() error {
	if a.Username == "" || a.Password.Empty() {
		return errors.New("incomplete auth type")
	}
	return nil
}

// HandleLogin returns a handler that signs a browser in.
//
// This is the browser's way in, as distinct from the GReader ClientLogin
// endpoint that API clients use. The difference is where the credential ends
// up: this one puts it in a cookie the page cannot read, while ClientLogin
// returns it in the body because a client like a feed reader has nowhere else
// to keep it.
func HandleLogin(d storage.Database) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var a auth
		decoder := json.NewDecoder(r.Body)

		if err := decoder.Decode(&a); err != nil {
			log.Warningf("Unable to decode auth: %s", err)
			returnError(w, r)
			return
		}
		defer r.Body.Close()

		if err := a.valid(); err != nil {
			log.Warningf("Incomplete login request: %s", err)
			returnError(w, r)
			return
		}

		u, err := d.GetUserByUsername(a.Username)
		if err != nil {
			log.Warningf("Failed to find user: %s", a.Username)
			returnLoginFailed(w, r)
			return
		}

		// Checked against the password hash rather than by looking the user up
		// by their derived key. The key is an unsalted MD5 of the username and
		// password, so matching on it is a far weaker test than the hash the
		// same password already has stored.
		if err = bcrypt.CompareHashAndPassword([]byte(u.HashPass.Reveal()), []byte(a.Password.Reveal())); err != nil {
			log.Warningf("Failed to validate password for user: %s", a.Username)
			returnLoginFailed(w, r)
			return
		}

		token, err := d.CreateSession(u, models.AuthSchemeWeb, r.Header.Get("User-Agent"))
		if err != nil {
			log.Warningf("Failed to create session: %s", err)
			returnError(w, r)
			return
		}

		setSessionCookie(w, r, token)
		returnSuccess(w, r)
	}
}

// HandleLogout returns a handler that ends a browser's session.
//
// The session is revoked rather than only forgotten, so that signing out
// actually withdraws the credential instead of leaving one behind that would
// still work if it were ever recovered.
func HandleLogout(d storage.Database) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Cleared first and unconditionally: a browser asking to sign out ends
		// up signed out whatever the server manages to do about the session.
		clearSessionCookie(w, r)

		token, ok := sessionFromCookie(r)
		if !ok {
			returnSuccess(w, r)
			return
		}

		user, session, err := d.LookupSession(token)
		if err != nil {
			// Already expired or revoked, which is the state being asked for.
			returnSuccess(w, r)
			return
		}

		if _, err = d.DeleteSessionForUser(user, session.SessionId); err != nil {
			log.Warningf("Failed to revoke session on logout: %s", err)
		}
		returnSuccess(w, r)
	}
}
