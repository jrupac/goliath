package storage

import (
	"testing"
	"time"
)

// The token is the entire credential, so two of them must never coincide and
// each must carry the full entropy the format promises.
func TestNewSessionTokenIsUnpredictable(t *testing.T) {
	const iterations = 1000

	seen := make(map[string]bool, iterations)
	for i := 0; i < iterations; i++ {
		token, err := newSessionToken()
		if err != nil {
			t.Fatalf("newSessionToken: %v", err)
		}
		if seen[token] {
			t.Fatalf("newSessionToken repeated %q", token)
		}
		seen[token] = true

		if !IsSessionToken(token) {
			t.Errorf("newSessionToken produced %q, which is not recognized as one", token)
		}
	}
}

// Credentials predating sessions must not be mistaken for session tokens; the
// authentication path picks which one to try on this alone.
func TestIsSessionTokenRejectsOtherCredentials(t *testing.T) {
	for _, token := range []string{
		"",
		"eyJ1c2VybmFtZSI6ImEifQ==",
		"d41d8cd98f00b204e9800998ecf8427e",
		"gol1",
		"GOL1_uppercase",
	} {
		if IsSessionToken(token) {
			t.Errorf("IsSessionToken(%q) = true, want false", token)
		}
	}
}

// Only the digest is stored, so it must be stable across calls and must not
// reveal the token.
func TestHashSessionTokenIsStableAndOpaque(t *testing.T) {
	token, err := newSessionToken()
	if err != nil {
		t.Fatalf("newSessionToken: %v", err)
	}

	first := hashSessionToken(token)
	if second := hashSessionToken(token); string(first) != string(second) {
		t.Error("hashSessionToken is not stable across calls")
	}

	other, err := newSessionToken()
	if err != nil {
		t.Fatalf("newSessionToken: %v", err)
	}
	if string(hashSessionToken(other)) == string(first) {
		t.Error("distinct tokens hashed to the same digest")
	}

	if len(first) != 32 {
		t.Errorf("digest is %d bytes, want 32", len(first))
	}
	for _, b := range first {
		if b != 0 {
			return
		}
	}
	t.Error("digest is all zeroes")
}

func TestSessionExpiryCutoffTracksIdleWindow(t *testing.T) {
	original := *sessionIdleWindow
	defer func() { *sessionIdleWindow = original }()

	*sessionIdleWindow = 90 * 24 * time.Hour
	want := time.Now().Add(-90 * 24 * time.Hour)

	if got := SessionExpiryCutoff(); got.Sub(want) > time.Minute || want.Sub(got) > time.Minute {
		t.Errorf("SessionExpiryCutoff() = %s, want about %s", got, want)
	}
}
