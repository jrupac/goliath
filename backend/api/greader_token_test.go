package api

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jrupac/goliath/models"
)

var (
	testUser    = models.User{UserId: "user-1", Username: "someone", Key: "k"}
	testSession = models.Session{SessionId: "session-1", UserId: testUser.UserId, Scheme: models.AuthSchemeGReader}
)

func TestPostTokenRoundTrips(t *testing.T) {
	binding := postTokenBinding(testUser, testSession)
	if !validatePostToken(binding, createPostToken(binding)) {
		t.Error("a freshly issued token did not validate")
	}
}

// A post token is worthless if it can be used by whoever finds it, so it must
// only work for the credential it was issued to.
func TestPostTokenIsBoundToItsCredential(t *testing.T) {
	otherSession := models.Session{SessionId: "session-2", UserId: testUser.UserId}
	otherUser := models.User{UserId: "user-2", Username: "someone-else", Key: "k"}

	token := createPostToken(postTokenBinding(testUser, testSession))

	for _, tc := range []struct {
		name    string
		binding string
	}{
		{"a different session for the same user", postTokenBinding(testUser, otherSession)},
		{"a different user", postTokenBinding(otherUser, models.Session{})},
		{"the same user without a session", postTokenBinding(testUser, models.Session{})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if validatePostToken(tc.binding, token) {
				t.Error("token validated against a credential it was not issued to")
			}
		})
	}
}

// Revoking a session has to take the post tokens issued under it with it, which
// is the whole reason the binding is the session rather than the user.
func TestPostTokenDiesWithItsSession(t *testing.T) {
	token := createPostToken(postTokenBinding(testUser, testSession))

	// The user is unchanged; only the session is gone.
	if validatePostToken(postTokenBinding(testUser, models.Session{}), token) {
		t.Error("token outlived the session it was issued under")
	}
}

// A credential predating sessions has none to bind to, so it binds to the user
// and still has to work.
func TestPostTokenWorksWithoutASession(t *testing.T) {
	binding := postTokenBinding(testUser, models.Session{})
	if !validatePostToken(binding, createPostToken(binding)) {
		t.Error("a token issued without a session did not validate")
	}
}

func TestPostTokenRejectsMalformedInput(t *testing.T) {
	binding := postTokenBinding(testUser, testSession)

	for _, token := range []string{
		"",
		"post_token", // what this replaced
		"nodot",
		".",
		"notanumber.YWJj",
		"1700000000.not-base64!!",
		"1700000000.",
		"1700000000",
	} {
		if validatePostToken(binding, token) {
			t.Errorf("validated malformed token %q", token)
		}
	}
}

// The issue time is readable, so a client could try to edit it. It is inside
// the signature precisely so that doing so does not work.
func TestPostTokenIssueTimeIsCoveredBySignature(t *testing.T) {
	binding := postTokenBinding(testUser, testSession)
	token := createPostToken(binding)

	issuedStr, mac, _ := strings.Cut(token, ".")
	issued, err := strconv.ParseInt(issuedStr, 10, 64)
	if err != nil {
		t.Fatalf("parsing the issue time: %v", err)
	}

	// Move it forward, which would otherwise buy the holder more time.
	forged := fmt.Sprintf("%d.%s", issued+3600, mac)
	if validatePostToken(binding, forged) {
		t.Error("a token with an edited issue time validated")
	}
}

func TestPostTokenExpires(t *testing.T) {
	binding := postTokenBinding(testUser, testSession)

	// Issued exactly at the far edge of the window, and just beyond it.
	within := forgeTokenIssuedAt(binding, time.Now().Add(-*postTokenTTL+time.Minute))
	if !validatePostToken(binding, within) {
		t.Error("a token inside the validity window was rejected")
	}

	expired := forgeTokenIssuedAt(binding, time.Now().Add(-*postTokenTTL-time.Minute))
	if validatePostToken(binding, expired) {
		t.Error("an expired token validated")
	}
}

// A token stamped far in the future is not something this server issued.
func TestPostTokenRejectsImplausibleFutureIssueTimes(t *testing.T) {
	binding := postTokenBinding(testUser, testSession)

	tolerated := forgeTokenIssuedAt(binding, time.Now().Add(postTokenClockSkew/2))
	if !validatePostToken(binding, tolerated) {
		t.Error("a token within the tolerated clock skew was rejected")
	}

	ahead := forgeTokenIssuedAt(binding, time.Now().Add(postTokenClockSkew+time.Minute))
	if validatePostToken(binding, ahead) {
		t.Error("a token issued implausibly far in the future validated")
	}
}

// forgeTokenIssuedAt builds a correctly signed token bearing an arbitrary issue
// time, which is the only way to reach the expiry logic without waiting.
func forgeTokenIssuedAt(binding string, at time.Time) string {
	issued := at.Unix()
	return fmt.Sprintf("%d.%s", issued,
		base64.RawURLEncoding.EncodeToString(postTokenMAC(binding, issued)))
}

// Requests are logged in full to make client behavior legible, which means the
// logging path is where credentials would otherwise end up in the clear.
func TestRedactFormValuesMasksCredentials(t *testing.T) {
	form := url.Values{}
	form.Add("Email", "someone")
	form.Add("Passwd", "hunter2")
	form.Add("api_key", "d7a649cda07c817e8a7f0b2ee0872fe3")
	form.Add(postTokenParam, "1788757481.2AhAOLEGV3FrcLrqJAUkUl8l_uUHcT7dnuUq07HwXYk")
	form.Add("i", "3039")

	got := redactFormValues(form)

	for _, secret := range []string{
		"hunter2",
		"d7a649cda07c817e8a7f0b2ee0872fe3",
		"2AhAOLEGV3FrcLrqJAUkUl8l_uUHcT7dnuUq07HwXYk",
	} {
		if strings.Contains(got, secret) {
			t.Errorf("leaked %q:\n%s", secret, got)
		}
	}

	// What is not a credential has to survive, or the log stops being useful.
	for _, kept := range []string{"someone", "3039", "1788757481"} {
		if !strings.Contains(got, kept) {
			t.Errorf("dropped %q:\n%s", kept, got)
		}
	}
}

// A post token keeps its issue time so that a rejection stays legible.
func TestRedactPostTokenKeepsIssueTime(t *testing.T) {
	if got, want := redactPostToken("1788757481.2AhAOLEG"), "1788757481."+redactedValue; got != want {
		t.Errorf("redactPostToken = %q, want %q", got, want)
	}
	// Nothing recognizable means nothing worth keeping.
	if got := redactPostToken("garbage"); got != redactedValue {
		t.Errorf("redactPostToken = %q, want %q", got, redactedValue)
	}
}
