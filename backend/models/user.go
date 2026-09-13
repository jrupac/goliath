package models

import (
	"crypto/md5"
	"errors"
	"fmt"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// MaxUsernameLength bounds a username in characters.
const MaxUsernameLength = 64

// usernamePattern is what a new username must match: ASCII letters and
// digits, with '.', '_' and '-' after the first character.
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidateUsername reports whether a name can be given to a new user.
//
// Names are typed by hand into clients, so they are kept to characters that
// look the same everywhere and have one spelling: with Unicode, two names can
// render identically, differ only in normalization, or fold case differently
// in the database than in Go. It also rules out a colon, which matters beyond
// looks: the Fever key is the digest of the username and password joined by
// one, so "a:b" with password "c" and "a" with password "b:c" would derive the
// same key.
func ValidateUsername(name string) error {
	switch {
	case name == "":
		return errors.New("username must not be empty")
	case len(name) > MaxUsernameLength:
		return fmt.Errorf("username must be at most %d characters", MaxUsernameLength)
	case !usernamePattern.MatchString(name):
		return errors.New("username must be ASCII letters and digits, with '.', '_' or '-' after the first character")
	}
	return nil
}

// NewUser returns a user with the given name and every credential derived from
// the password, ready to be stored. The ID is left for the database to assign.
func NewUser(username string, password Secret) (User, error) {
	if err := ValidateUsername(username); err != nil {
		return User{}, err
	}
	if password.Empty() {
		return User{}, errors.New("password must not be empty")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password.Reveal()), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("could not hash password: %w", err)
	}
	return User{
		Username: username,
		Key:      DeriveUserKey(username, password),
		HashPass: Secret(hashed),
	}, nil
}

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

// UserSummary is a user along with counts of what they hold.
type UserSummary struct {
	User User
	// Feeds counts live subscriptions, and Folders every folder but the root.
	Feeds   int64
	Folders int64
	// Articles counts those in live feeds, and Unread those of them unread.
	Articles int64
	Unread   int64
	// Sessions counts unexpired sessions.
	Sessions int64
	// Deleted is when the user was deleted, or zero if they have not been.
	Deleted time.Time
}

// UserDeletion counts what went with a deleted user.
type UserDeletion struct {
	Feeds    int64
	Articles int64
	Sessions int64
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
