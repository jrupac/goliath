package api

import (
	"context"

	"github.com/jrupac/goliath/models"
)

// contextKey is unexported so that nothing outside this package can collide
// with the values stored under it.
type contextKey int

const sessionContextKey contextKey = iota

// withSession returns a context carrying the session a request authenticated
// with.
//
// The session travels on the request rather than in the handler signature
// because only the handlers that issue or check a post token need it, and
// threading it through every handler to reach those would put it in a lot of
// places that have no use for it.
func withSession(ctx context.Context, session models.Session) context.Context {
	return context.WithValue(ctx, sessionContextKey, session)
}

// sessionFrom returns the session a request authenticated with, or the zero
// session if it authenticated some other way.
func sessionFrom(ctx context.Context) models.Session {
	session, _ := ctx.Value(sessionContextKey).(models.Session)
	return session
}
