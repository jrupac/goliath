package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
)

func createPostToken() string {
	// TODO: Support short-lived POST tokens
	return "post_token"
}

func validatePostToken(token string) bool {
	// TODO: Support short-lived POST tokens
	return token == "post_token"
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
func validateLegacyAuthToken(token []byte, username string, hashPass string) bool {
	h := sha256.New()
	h.Write([]byte(legacyTokenSalt))
	h.Write([]byte(username))
	h.Write([]byte(hashPass))

	return bytes.Equal(token, h.Sum(nil))
}
