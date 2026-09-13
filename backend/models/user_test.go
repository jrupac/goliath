package models

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestValidateUsername(t *testing.T) {
	valid := []string{"alice", "Alice", "tester01", "first.last", "a_b-c", "7", strings.Repeat("a", MaxUsernameLength)}
	for _, name := range valid {
		if err := ValidateUsername(name); err != nil {
			t.Errorf("ValidateUsername(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"",
		strings.Repeat("a", MaxUsernameLength+1),
		"a:b",
		"Alice Smith",
		" alice",
		"alice\t",
		"al\x00ice",
		".alice",
		"-alice",
		"ünïcode",
		"аlice", // Cyrillic а
		"alice@example.com",
		"\xff\xfe",
	}
	for _, name := range invalid {
		if err := ValidateUsername(name); err == nil {
			t.Errorf("ValidateUsername(%q) = nil, want an error", name)
		}
	}
}

// A colon in the username is what would let two accounts derive one key.
func TestColonInUsernameWouldCollideFeverKeys(t *testing.T) {
	if DeriveUserKey("a:b", Secret("c")) != DeriveUserKey("a", Secret("b:c")) {
		t.Fatal("expected the keys to collide, which is why a colon is refused")
	}
}

func TestNewUserDerivesEveryCredential(t *testing.T) {
	u, err := NewUser("alice", Secret("hunter2"))
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	if u.Username != "alice" || u.UserId != "" {
		t.Errorf("got %+v, want username alice and no ID", u)
	}
	if u.Key != DeriveUserKey("alice", Secret("hunter2")) {
		t.Error("key is not derived from the username and password")
	}
	if err = bcrypt.CompareHashAndPassword([]byte(u.HashPass.Reveal()), []byte("hunter2")); err != nil {
		t.Errorf("hash does not match the password: %v", err)
	}
}

func TestNewUserRefusals(t *testing.T) {
	if _, err := NewUser("a:b", Secret("pw")); err == nil {
		t.Error("accepted a username with a colon")
	}
	if _, err := NewUser("alice", Secret("")); err == nil {
		t.Error("accepted an empty password")
	}
}
