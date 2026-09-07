package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
)

var (
	postTokenKey = flag.String("postTokenKey", "",
		"Secret used to sign GReader post tokens. If unset, a random key is generated at "+
			"startup, which invalidates outstanding post tokens on every restart.")
	postTokenTTL = flag.Duration("postTokenTTL", 1*time.Hour,
		"Duration a GReader post token remains valid after being issued.")
)

// postTokenClockSkew is how far in the future a token's issue time may sit and
// still be accepted, so that a client is not locked out by a small disagreement
// about the current time.
const postTokenClockSkew = 5 * time.Minute

// postTokenSigningKey is resolved once, at startup.
var postTokenSigningKey []byte

// InitPostTokenKey establishes the key post tokens are signed with. It must be
// called after flags are parsed and before any request is served.
//
// There is deliberately no compiled-in default. A shared constant would be
// public knowledge and so no protection at all, which is the flaw that makes
// the legacy auth token below worth replacing. Generating a key instead costs
// only that tokens do not survive a restart, and clients already know how to
// fetch a new one.
func InitPostTokenKey() {
	if *postTokenKey != "" {
		postTokenSigningKey = []byte(*postTokenKey)
		return
	}

	postTokenSigningKey = make([]byte, 32)
	if _, err := rand.Read(postTokenSigningKey); err != nil {
		// Not an expected path: the reader either fills the buffer or takes
		// the process down itself, so this never returns an error in practice.
		// Handled anyway rather than ignored, because the alternative to
		// stopping here is signing with a key that is partly or wholly zero,
		// which would look like it was working.
		log.Fatalf("Could not generate a post token signing key: %s", err)
	}
	log.Warningf("No postTokenKey configured; generated one. Post tokens will not " +
		"survive a restart.")
}

// postTokenBinding returns what a post token issued for this credential is tied
// to.
//
// Binding to the session means the token stops working the moment the session
// does, with nothing to expire separately. A credential that predates sessions
// has none, so those bind to the user instead and last as long as that way of
// authenticating does.
func postTokenBinding(user models.User, session models.Session) string {
	if session.SessionId != "" {
		return "session:" + string(session.SessionId)
	}
	return "user:" + string(user.UserId)
}

func postTokenMAC(binding string, issued int64) []byte {
	mac := hmac.New(sha256.New, postTokenSigningKey)
	// The issue time is inside the MAC, so a client cannot extend its own
	// token's life by editing the part it can read.
	fmt.Fprintf(mac, "%s.%d", binding, issued)
	return mac.Sum(nil)
}

// createPostToken issues a post token for the given credential.
func createPostToken(binding string) models.Secret {
	issued := time.Now().Unix()
	return models.Secret(fmt.Sprintf("%d.%s", issued,
		base64.RawURLEncoding.EncodeToString(postTokenMAC(binding, issued))))
}

// validatePostToken reports whether a post token was issued for this credential
// and has not expired.
func validatePostToken(binding string, token models.Secret) bool {
	issuedStr, macStr, found := strings.Cut(token.Reveal(), ".")
	if !found {
		return false
	}

	issued, err := strconv.ParseInt(issuedStr, 10, 64)
	if err != nil {
		return false
	}

	mac, err := base64.RawURLEncoding.DecodeString(macStr)
	if err != nil {
		return false
	}

	// Checked before the issue time is trusted for anything, since it is only
	// authentic once it has been covered by a signature that verifies.
	if !hmac.Equal(mac, postTokenMAC(binding, issued)) {
		return false
	}

	age := time.Since(time.Unix(issued, 0))
	return age <= *postTokenTTL && age >= -postTokenClockSkew
}

/*******************************************************************************
 * Legacy auth tokens
 *
 * Superseded by server-issued session tokens, which expire and can be revoked.
 * These are still accepted so that clients holding one keep working: the token
 * is stored on the client indefinitely and there is no refresh mechanism in
 * this API, so rejecting it means the user re-entering their password on every
 * device at once. Nothing issues this format any more, and the whole section
 * comes out once the deprecation window closes.
 ******************************************************************************/

// legacyTokenSalt is a fixed, compiled-in salt. That it cannot be rotated is
// one of the reasons this format was replaced.
const legacyTokenSalt = "D5qpSQLaDlGbbcKxj2Jj0Q=="

// extractLegacyAuthToken splits a legacy token into the username it names and
// the digest it carries.
//
// The token format is a base64 encoding of a JSON object containing:
//
//	{
//	  Username: username,
//	  Token: base64(sha256(legacyTokenSalt + username + bcrypt(salt + password)))
//	}
func extractLegacyAuthToken(token string) (string, []byte, error) {
	var t greaderTokenType

	jsn, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", []byte{}, err
	}

	err = json.Unmarshal(jsn, &t)
	if err != nil {
		return "", []byte{}, err
	}

	dec, err := base64.URLEncoding.DecodeString(t.Token)
	if err != nil {
		return "", []byte{}, err
	}

	return t.Username, dec, nil
}

// validateLegacyAuthToken reports whether a legacy token matches the given
// user. The token is derived from the password hash, so it is equivalent to it
// in blast radius and stays valid until the password changes.
func validateLegacyAuthToken(token []byte, username string, hashPass models.Secret) bool {
	h := sha256.New()
	h.Write([]byte(legacyTokenSalt))
	h.Write([]byte(username))
	h.Write([]byte(hashPass.Reveal()))

	return bytes.Equal(token, h.Sum(nil))
}
