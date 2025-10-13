package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
)

// This is a fixed, randomly generated salt used as part of the generated
// auth token. This is temporary until a token with expiration support is
// implemented (i.e., with JWT).
const tokenSalt = "D5qpSQLaDlGbbcKxj2Jj0Q=="

func createPostToken() string {
	// TODO: Support short-lived POST tokens
	return "post_token"
}

func validatePostToken(token string) bool {
	// TODO: Support short-lived POST tokens
	return token == "post_token"
}

func createAuthToken(hashPass string, username string) (string, error) {
	// This roughly follows what FreshRSS uses for the auth token
	// The token format is a base64 encoding of a JSON object containing:
	// {
	//   Username: username,
	//   Token: base64(sha256(tokenSalt + username + bcrypt(salt + password)))
	// }
	h := sha256.New()
	h.Write([]byte(tokenSalt))
	h.Write([]byte(username))
	h.Write([]byte(hashPass))
	t := h.Sum(nil)

	token := greaderTokenType{
		Username: username,
		Token:    base64.URLEncoding.EncodeToString(t),
	}
	t, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(t), nil
}

func extractAuthToken(token string) (string, []byte, error) {
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

func validateAuthToken(token []byte, username string, hashPass string) bool {
	h := sha256.New()
	h.Write([]byte(tokenSalt))
	h.Write([]byte(username))
	h.Write([]byte(hashPass))

	return bytes.Equal(token, h.Sum(nil))
}
