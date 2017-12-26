package models

// User is a single user of the application.
type User struct {
	Username string
	Key      string
}

// Valid returns true if this object is well-formed.
func (u *User) Valid() bool {
	return u.Username != "" && u.Key != ""
}
