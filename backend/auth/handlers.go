package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
)

type auth struct {
	Username string        `json:"username"`
	Password models.Secret `json:"password"`
}

func (a *auth) getAPIKey() (models.Secret, error) {
	if a.Username == "" || a.Password.Empty() {
		return "", errors.New("incomplete auth type")
	}
	return models.DeriveUserKey(a.Username, a.Password), nil
}

// HandleLogin returns a handler that implements logging into the application.
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

		key, err := a.getAPIKey()
		if err != nil {
			log.Warningf("Unable to compute key: %s", err)
			returnError(w, r)
			return
		}

		// Do actual login check here.
		u, err := d.GetUserByKey(key)
		if err != nil {
			returnLoginFailed(w, r)
			return
		}

		c := http.Cookie{
			Name: authCookie,
			// Revealed at the boundary: the cookie is how this credential
			// reaches the client, so it is a deliberate handoff.
			Value: u.Key.Reveal(),
		}
		http.SetCookie(w, &c)
		returnSuccess(w, r)
	}
}

// HandleLogout handles logging out of the application.
// NOTE: This is not yet implemented.
func HandleLogout(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
