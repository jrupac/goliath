package auth

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/storage"
	"net/http"
)

type auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *auth) getAPIKey() (string, error) {
	if a.Username == "" || a.Password == "" {
		return "", errors.New("incomplete auth type")
	}
	key := md5.Sum([]byte(fmt.Sprintf("%s:%s", a.Username, a.Password)))
	return fmt.Sprintf("%x", string(key[:])), nil
}

// HandleLogin returns a handler that implements logging into the application.
func HandleLogin(d *storage.Database) func(http.ResponseWriter, *http.Request) {
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

		u, err2 := d.GetUserByKey(key)
		if err2 != nil {
			returnError(w, r)
			return
		}
		c := http.Cookie{
			Name:  authCookie,
			Value: u.Key,
		}
		http.SetCookie(w, &c)
		returnSuccess(w, r)
		return
	}
}

// HandleLogout handles logging out of the application.
// NOTE: This is not yet implemented.
func HandleLogout(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
