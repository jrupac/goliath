package models

type User struct {
	Username string
	Key      string
}

func (u *User) Valid() bool {
	return u.Username != "" && u.Key != ""
}
