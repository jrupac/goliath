package models

import (
	"fmt"
	"time"
)

// SessionId is a unique reference to a single authenticated session.
type SessionId string

// AuthScheme names the authentication surface that issued a session.
//
// Sessions are shared across surfaces so that one table answers "who is logged
// in" for the whole application, but the surfaces differ in what they can use:
// a scheme appears here only if the server chooses the credential it hands out.
// Fever is therefore absent by construction — its API key is derived by the
// client from the username and password, so the server never issues one.
type AuthScheme string

const (
	// AuthSchemeGReader is a session established through the Google Reader
	// ClientLogin endpoint and presented as a bearer token.
	AuthSchemeGReader AuthScheme = "greader"
	// AuthSchemeWeb is a session established through the web login form and
	// presented as a cookie.
	AuthSchemeWeb AuthScheme = "web"
)

// Session is a single authenticated login, held by one client.
//
// The bearer token itself is deliberately not a field: it is generated once,
// returned to the client, and thereafter known to the server only by its
// digest.
type Session struct {
	// Primary key
	SessionId SessionId
	UserId    UserId
	// Scheme records which surface issued this session, so that a listing can
	// distinguish a browser login from a feed reader.
	Scheme  AuthScheme
	Created time.Time
	// LastSeen advances as the session is used and is what expiry is measured
	// against.
	LastSeen time.Time
	// UserAgent is recorded at creation to make a session recognizable to the
	// person deciding whether to revoke it. It is client-supplied and may be
	// absent or untrue.
	UserAgent string
}

// Valid returns true if this object is well-formed.
func (s Session) Valid() bool {
	return s.SessionId != "" && s.UserId != "" && s.Scheme != ""
}

func (s Session) String() string {
	return fmt.Sprintf("Session{SessionId:\"%s\", UserId:\"%s\", Scheme:\"%s\"}", s.SessionId, s.UserId, s.Scheme)
}
