package models

import "fmt"

// UserId is a unique reference to a single user in the system.
type UserId string

// User is a single user of the application.
type User struct {
	// Primary key
	UserId   UserId
	Username string
	Key      string
	HashPass string
}

// Valid returns true if this object is well-formed.
func (u User) Valid() bool {
	return u.UserId != "" && u.Username != "" && u.Key != ""
}

func (u User) String() string {
	return fmt.Sprintf("User{UserId:\"%s\"}", u.UserId)
}
