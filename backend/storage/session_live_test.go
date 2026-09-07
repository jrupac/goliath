package storage

import (
	"os"
	"testing"
	"time"

	"github.com/jrupac/goliath/models"
)

// Exercises the session queries against a real CockroachDB. Skipped unless
// GOLIATH_TEST_DB names one, since it writes.
func TestSessionLifecycleAgainstDatabase(t *testing.T) {
	dsn := os.Getenv("GOLIATH_TEST_DB")
	if dsn == "" {
		t.Skip("GOLIATH_TEST_DB not set")
	}

	crdb := &Crdb{}
	if err := crdb.Open(dsn); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer crdb.Close()

	users, err := crdb.GetAllUsers()
	if err != nil || len(users) == 0 {
		t.Fatalf("GetAllUsers: %v (%d users)", err, len(users))
	}
	u, err := crdb.GetUserByUsername(users[0].Username)
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}

	if _, err = crdb.DeleteSessionsForUser(u); err != nil {
		t.Fatalf("DeleteSessionsForUser: %v", err)
	}

	token, err := crdb.CreateSession(u, models.AuthSchemeGReader, "TestClient/1.0")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if !IsSessionToken(token) {
		t.Fatalf("CreateSession returned %q", token)
	}

	gotUser, gotSession, err := crdb.LookupSession(token)
	if err != nil {
		t.Fatalf("LookupSession: %v", err)
	}
	if gotUser.UserId != u.UserId || gotUser.Username != u.Username {
		t.Errorf("LookupSession returned user %v, want %v", gotUser, u)
	}
	if gotSession.Scheme != models.AuthSchemeGReader {
		t.Errorf("scheme = %q, want %q", gotSession.Scheme, models.AuthSchemeGReader)
	}
	if gotSession.UserAgent != "TestClient/1.0" {
		t.Errorf("user agent = %q", gotSession.UserAgent)
	}
	if time.Since(gotSession.Created) > time.Minute {
		t.Errorf("created = %s, want about now", gotSession.Created)
	}

	if _, _, err = crdb.LookupSession(token + "x"); err == nil {
		t.Error("LookupSession accepted a token that was never issued")
	}

	sessions, err := crdb.GetSessionsForUser(u)
	if err != nil {
		t.Fatalf("GetSessionsForUser: %v", err)
	}
	if len(sessions) != 1 || sessions[0].SessionId != gotSession.SessionId {
		t.Fatalf("GetSessionsForUser returned %v, want just %s", sessions, gotSession.SessionId)
	}

	// Revocation takes effect immediately, with no cached decision to outlive it.
	n, err := crdb.DeleteSessionForUser(u, gotSession.SessionId)
	if err != nil || n != 1 {
		t.Fatalf("DeleteSessionForUser: %v (deleted %d)", err, n)
	}
	if _, _, err = crdb.LookupSession(token); err == nil {
		t.Error("LookupSession accepted a revoked session")
	}

	// Using a session slides its expiry, but only once per touch interval:
	// authenticating is a read, and every request must not turn it into a write.
	if token, err = crdb.CreateSession(u, models.AuthSchemeGReader, "toucher"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	_, fresh, err := crdb.LookupSession(token)
	if err != nil {
		t.Fatalf("LookupSession: %v", err)
	}
	if _, unchanged, err := crdb.LookupSession(token); err != nil {
		t.Fatalf("LookupSession: %v", err)
	} else if !unchanged.LastSeen.Equal(fresh.LastSeen) {
		t.Errorf("last seen advanced on a second immediate lookup: %s then %s", fresh.LastSeen, unchanged.LastSeen)
	}

	aged := time.Now().Add(-*sessionTouchInterval - time.Minute)
	if _, err = crdb.db.Exec(`UPDATE Session SET lastseen = $1 WHERE id = $2`, aged, fresh.SessionId); err != nil {
		t.Fatalf("aging the session: %v", err)
	}
	if _, _, err = crdb.LookupSession(token); err != nil {
		t.Fatalf("LookupSession: %v", err)
	}
	var slid time.Time
	if err = crdb.db.QueryRow(`SELECT lastseen FROM Session WHERE id = $1`, fresh.SessionId).Scan(&slid); err != nil {
		t.Fatalf("reading back last seen: %v", err)
	}
	if !slid.After(aged) {
		t.Errorf("last seen did not slide: still %s", slid)
	}
	if _, err = crdb.DeleteSessionForUser(u, fresh.SessionId); err != nil {
		t.Fatalf("DeleteSessionForUser: %v", err)
	}

	// A session belongs to its user: another user's ID must not revoke it.
	if token, err = crdb.CreateSession(u, models.AuthSchemeWeb, "other"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	_, s2, err := crdb.LookupSession(token)
	if err != nil {
		t.Fatalf("LookupSession: %v", err)
	}
	stranger := models.User{UserId: "00000000-0000-0000-0000-000000000000", Username: "x", Key: "x"}
	if n, err = crdb.DeleteSessionForUser(stranger, s2.SessionId); err != nil || n != 0 {
		t.Errorf("DeleteSessionForUser by a different user deleted %d rows (err %v), want 0", n, err)
	}

	// Expiry is enforced on lookup, not only by the sweep.
	if _, err = crdb.db.Exec(
		`UPDATE Session SET lastseen = $1 WHERE id = $2`,
		time.Now().Add(-*sessionIdleWindow-time.Hour), s2.SessionId); err != nil {
		t.Fatalf("aging the session: %v", err)
	}
	if _, _, err = crdb.LookupSession(token); err == nil {
		t.Error("LookupSession accepted a session past the idle window")
	}
	if sessions, err = crdb.GetSessionsForUser(u); err != nil || len(sessions) != 0 {
		t.Errorf("GetSessionsForUser listed %d expired sessions (err %v)", len(sessions), err)
	}
	if n, err = crdb.DeleteExpiredSessions(SessionExpiryCutoff()); err != nil || n != 1 {
		t.Errorf("DeleteExpiredSessions swept %d rows (err %v), want 1", n, err)
	}
}
