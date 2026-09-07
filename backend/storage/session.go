package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"strings"
	"time"
)

var (
	sessionIdleWindow = flag.Duration("sessionIdleWindow", 90*24*time.Hour,
		"Duration a session may go unused before it expires.")
	sessionTouchInterval = flag.Duration("sessionTouchInterval", 1*time.Hour,
		"Minimum duration between writes recording that a session was used.")
)

const (
	// sessionTokenPrefix marks a server-issued session token. Tokens are opaque
	// to clients, but a fixed prefix keeps them recognizable in a log or a
	// support request, and distinguishes them from credential formats that
	// predate sessions without having to parse anything.
	sessionTokenPrefix = "gol1_"
	// sessionTokenBytes is the entropy behind a token. The token is the entire
	// credential, so it must be far beyond guessing; 256 bits also means the
	// digest below needs no salt or key stretching, since there is no
	// lower-entropy secret hiding underneath it.
	sessionTokenBytes = 32
)

// newSessionToken returns a fresh bearer token. The caller is responsible for
// handing it to the client, which is the only time it can be recovered.
func newSessionToken() (string, error) {
	b := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("could not generate session token: %w", err)
	}
	return sessionTokenPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// IsSessionToken reports whether a credential was issued as a session token.
// It says nothing about whether that session exists or is still valid.
func IsSessionToken(token string) bool {
	return strings.HasPrefix(token, sessionTokenPrefix)
}

// hashSessionToken maps a bearer token to what is stored for it.
func hashSessionToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// SessionExpiryCutoff returns the instant before which a session is considered
// expired. A session last used after this is still valid.
func SessionExpiryCutoff() time.Time {
	return time.Now().Add(-*sessionIdleWindow)
}
