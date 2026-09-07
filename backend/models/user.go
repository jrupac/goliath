package models

import (
	"crypto/md5"
	"fmt"
)

// UserId is a unique reference to a single user in the system.
type UserId string

// User is a single user of the application.
type User struct {
	// Primary key
	UserId   UserId
	Username string
	// Key is the Fever API key and the value carried by the web session
	// cookie. It is derived from the password, so it is a credential in its
	// own right rather than an identifier.
	Key Secret
	// HashPass is the bcrypt hash of the password. Tokens predating sessions
	// are derived from it, which makes it password-equivalent in reach.
	HashPass Secret
}

// Valid returns true if this object is well-formed.
func (u User) Valid() bool {
	return u.UserId != "" && u.Username != "" && !u.Key.Empty()
}

func (u User) String() string {
	return fmt.Sprintf("User{UserId:\"%s\"}", u.UserId)
}

// DeriveUserKey returns the value stored in a user's key column, which is what
// the Fever API accepts as its api_key and what the web session cookie
// carries.
//
// The formula is fixed by the Fever protocol: the client computes this itself
// from the username and password, so the server cannot choose it and cannot
// make it any stronger. It also means the key changes whenever the password
// does, and that anything holding the old one stops working.
func DeriveUserKey(username string, password Secret) Secret {
	return Secret(fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s", username, password.Reveal())))))
}
