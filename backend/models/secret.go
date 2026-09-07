package models

import (
	"database/sql/driver"
	"fmt"
	"io"
)

// redactedSecret is what a Secret renders as wherever it is printed.
const redactedSecret = "[REDACTED]"

// Secret is a credential carried as a string.
//
// It does two things. Printing one anywhere — a log line, an error, a dumped
// struct — yields a placeholder rather than the value, so a credential cannot
// reach a log merely by being handed to the wrong format call. And being a
// distinct type, it cannot be passed where a plain string is expected, so two
// adjacent string parameters cannot be swapped without the compiler objecting.
//
// It is a guardrail, not a guarantee: Reveal and a conversion back to string
// both defeat it, deliberately. Those are the lines worth reading closely.
//
// One verb escapes it. %p is resolved before any formatting method is
// consulted, and on a value that is not a pointer it falls through to printing
// the value itself. Nothing here can intercept that. Closing it would mean
// holding the value behind a pointer, which would in turn make == compare
// identity rather than contents — a far worse trap for a credential than a
// misapplied verb that announces itself as %!p(...).
type Secret string

// Format renders a Secret for every verb.
//
// Implementing fmt.Stringer alone is not enough. Both %#v and %d go around it
// and print the underlying value, as does %#v on a struct that merely has a
// Secret among its fields. fmt.Formatter is what makes the redaction hold
// everywhere rather than for the common verbs.
func (s Secret) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, redactedSecret)
}

// String satisfies fmt.Stringer, for callers that reach for it directly.
func (s Secret) String() string {
	return redactedSecret
}

// MarshalText keeps a Secret out of anything encoded from it.
func (s Secret) MarshalText() ([]byte, error) {
	return []byte(redactedSecret), nil
}

// MarshalJSON keeps a Secret out of a JSON response it was not meant to be in.
// Somewhere a credential is genuinely meant to be serialized — a login
// response hands one to the client — and that has to say Reveal explicitly.
func (s Secret) MarshalJSON() ([]byte, error) {
	return []byte(`"` + redactedSecret + `"`), nil
}

// Reveal returns the underlying value. Every call is a place where a credential
// escapes into ordinary string handling.
func (s Secret) Reveal() string {
	return string(s)
}

// Empty reports whether this holds no value, which callers otherwise could not
// ask without revealing it.
func (s Secret) Empty() bool {
	return s == ""
}

// Scan lets a Secret be read straight out of a database row.
func (s *Secret) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*s = ""
	case string:
		*s = Secret(v)
	case []byte:
		*s = Secret(v)
	default:
		// Deliberately names only the type: the value is a credential.
		return fmt.Errorf("cannot scan %T into Secret", src)
	}
	return nil
}

// Value lets a Secret be written straight into a database row.
func (s Secret) Value() (driver.Value, error) {
	return string(s), nil
}
