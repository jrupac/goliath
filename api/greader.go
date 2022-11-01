package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
	"time"
)

// This is a fixed, randomly generated salt used as part of the generated
// auth token. This is temporary until a token with expiration support is
// implemented (i.e., with JWT).
const tokenSalt = "D5qpSQLaDlGbbcKxj2Jj0Q=="

type tokenType struct {
	Username string `json:"username"`
	Token    string `json:"token"`
}

var (
	greaderLatencyMetric = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "greader_server_latency",
			Help:       "Server-side latency of GReader API operations.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"method"},
	)
)

func init() {
	prometheus.MustRegister(greaderLatencyMetric)
}

// GReader is an implementation of the GReader API.
type GReader struct {
	d *storage.Database
}

type greaderResponse map[string]string

// GReaderHandler returns a new GReader handler.
func GReaderHandler(d *storage.Database) http.HandlerFunc {
	return GReader{d}.Handler()
}

// Handler returns a handler function that implements the GReader API.
func (a GReader) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.route(w, r)
	}
}

func (a GReader) recordLatency(t time.Time, label string) {
	utils.Elapsed(t, func(d time.Duration) {
		// Record latency measurements in microseconds.
		greaderLatencyMetric.WithLabelValues(label).Observe(float64(d) / float64(time.Microsecond))
	})
}

func (a GReader) route(w http.ResponseWriter, r *http.Request) {
	// Record the total server latency of each call.
	defer a.recordLatency(time.Now(), "server")

	var resp greaderResponse
	var status int

	switch {
	case r.URL.Path == "/greader/accounts/ClientLogin":
		resp, status = a.handleLogin(w, r)
	case r.URL.Path == "/greader/reader/api/0/user-info":
		resp, status = a.withAuth(r, a.handleUserInfo)
	default:
		log.Infof("DID NOT MATCH ANY ROUTE")
		log.Infof("GReader request URL path: %s", r.URL.Path)
		log.Infof("GReader request URL: %s", r.URL.String())
		log.Infof("GReader request body: %s", r.PostForm.Encode())
		a.returnError(w, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if status != http.StatusOK {
		a.returnError(w, status)
	} else {
		a.returnSuccess(w, resp)
	}
}

func (a GReader) handleLogin(_ http.ResponseWriter, r *http.Request) (resp greaderResponse, status int) {
	resp = map[string]string{}
	status = http.StatusOK

	err := r.ParseForm()
	if err != nil {
		status = http.StatusInternalServerError
		return
	}

	formUser := r.PostForm.Get("Email")
	formPass := r.PostForm.Get("Passwd")

	user, err := a.d.GetUserByUsername(formUser)
	if err != nil {
		status = http.StatusInternalServerError
		return
	}

	ok := bcrypt.CompareHashAndPassword([]byte(user.HashPass), []byte(formPass))
	if ok == nil {
		token, err := createToken(user.HashPass, formUser)
		if err != nil {
			status = http.StatusInternalServerError
			return
		}

		resp["SID"] = ""
		resp["LSID"] = ""
		resp["Auth"] = token
	} else {
		status = http.StatusUnauthorized
		return
	}
	return
}

func (a GReader) handleUserInfo(r *http.Request, user models.User) (resp greaderResponse, status int) {
	resp = map[string]string{}
	status = http.StatusOK

	err := r.ParseForm()
	if err != nil {
		status = http.StatusInternalServerError
		return
	}

	resp["userId"] = string(user.UserId)
	resp["userName"] = user.Username

	return
}

func (a GReader) withAuth(r *http.Request, handler func(*http.Request, models.User) (greaderResponse, int)) (greaderResponse, int) {
	resp := map[string]string{}

	// Header should be in format:
	//   Authorization: GoogleLogin auth=<token>
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return resp, http.StatusUnauthorized
	}

	authFields := strings.Fields(authHeader)
	if len(authFields) != 2 || !strings.EqualFold(authFields[0], "GoogleLogin") {
		return resp, http.StatusBadRequest
	}

	authStr, tokenStr, found := strings.Cut(authFields[1], "=")
	if !found {
		return resp, http.StatusBadRequest
	}

	if tokenStr == "" || !strings.EqualFold(authStr, "auth") {
		return resp, http.StatusBadRequest
	}

	username, token, err := extractToken(tokenStr)
	if err != nil {
		return resp, http.StatusBadRequest
	}

	user, err := a.d.GetUserByUsername(username)
	if err != nil {
		return resp, http.StatusUnauthorized
	}

	if validateToken(token, username, user.HashPass) {
		return handler(r, user)
	}
	return resp, http.StatusUnauthorized
}

func (a GReader) returnError(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}

func (a GReader) returnSuccess(w http.ResponseWriter, resp greaderResponse) {
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(resp); err != nil {
		a.returnError(w, http.StatusInternalServerError)
	}
}

func createToken(hashPass string, username string) (string, error) {
	// This roughly follows what FreshRSS uses for the auth token
	// The token format is a JSON object containing:
	// {
	//   Username: username,
	//   Token: base64(sha256(tokenSalt + username + bcrypt(salt + password)))
	// }
	h := sha256.New()
	h.Write([]byte(tokenSalt))
	h.Write([]byte(username))
	h.Write([]byte(hashPass))
	t := h.Sum(nil)

	token := tokenType{
		Username: username,
		Token:    base64.URLEncoding.EncodeToString(t),
	}
	t, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	return string(t), nil
}

func extractToken(token string) (string, []byte, error) {
	var t tokenType
	err := json.Unmarshal([]byte(token), &t)
	if err != nil {
		return "", []byte{}, err
	}

	dec, err := base64.URLEncoding.DecodeString(t.Token)
	if err != nil {
		return "", []byte{}, err
	}

	return t.Username, dec, nil
}

func validateToken(token []byte, username string, hashPass string) bool {
	h := sha256.New()
	h.Write([]byte(tokenSalt))
	h.Write([]byte(username))
	h.Write([]byte(hashPass))

	return bytes.Compare(token, h.Sum(nil)) == 0
}
