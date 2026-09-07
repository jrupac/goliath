package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const probe = "s3cr3t-value"

// The point of the type is that no formatting verb prints the value. %#v and %d
// are the ones that matter: both go around fmt.Stringer, so a type that only
// implemented String would leak through them.
func TestSecretIsRedactedUnderEveryVerb(t *testing.T) {
	s := Secret(probe)

	for _, verb := range []string{"%s", "%v", "%+v", "%#v", "%q", "%x", "%X", "%d", "%t"} {
		got := fmt.Sprintf(verb, s)
		if strings.Contains(got, probe) {
			t.Errorf("%s leaked the value: %s", verb, got)
		}
		if !strings.Contains(got, redactedSecret) {
			t.Errorf("%s = %q, want it to contain %q", verb, got, redactedSecret)
		}
	}
}

// %p is the one verb that gets through, because it is resolved before any
// formatting method is consulted. Asserted so that the gap is a known one
// rather than something to be discovered in a log, and so that a future
// version of fmt closing it does not pass unnoticed.
func TestSecretLeaksOnlyUnderPointerVerb(t *testing.T) {
	if got := fmt.Sprintf("%p", Secret(probe)); !strings.Contains(got, probe) {
		t.Errorf("%%p on a Secret no longer prints the value (%s); the documented gap has closed", got)
	}
}

// A Secret is usually reached as a field of something larger, and dumping that
// larger thing is exactly how a credential ends up in a log.
func TestSecretIsRedactedInsideAStruct(t *testing.T) {
	holder := struct {
		Name  string
		Token Secret
	}{Name: "someone", Token: Secret(probe)}

	for _, verb := range []string{"%v", "%+v", "%#v"} {
		got := fmt.Sprintf(verb, holder)
		if strings.Contains(got, probe) {
			t.Errorf("%s leaked the value: %s", verb, got)
		}
	}

	// A pointer to the struct is just as common a thing to log.
	if got := fmt.Sprintf("%+v", &holder); strings.Contains(got, probe) {
		t.Errorf("pointer dump leaked the value: %s", got)
	}
}

func TestSecretIsRedactedWhenSerialized(t *testing.T) {
	encoded, err := json.Marshal(struct {
		Token Secret `json:"token"`
	}{Secret(probe)})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), probe) {
		t.Errorf("JSON leaked the value: %s", encoded)
	}

	text, err := Secret(probe).MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	if string(text) != redactedSecret {
		t.Errorf("MarshalText = %q, want %q", text, redactedSecret)
	}
}

// Redaction is only useful if the value is still reachable on purpose.
func TestSecretRevealReturnsTheValue(t *testing.T) {
	if got := Secret(probe).Reveal(); got != probe {
		t.Errorf("Reveal = %q, want %q", got, probe)
	}
	if !Secret("").Empty() {
		t.Error("an empty Secret does not report itself empty")
	}
	if Secret(probe).Empty() {
		t.Error("a non-empty Secret reports itself empty")
	}
}

// A Secret is stored in and read from the database like the string it is.
func TestSecretRoundTripsThroughSQL(t *testing.T) {
	value, err := Secret(probe).Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	if value != probe {
		t.Errorf("Value = %v, want %q", value, probe)
	}

	for _, src := range []any{probe, []byte(probe)} {
		var scanned Secret
		if err = scanned.Scan(src); err != nil {
			t.Fatalf("Scan(%T): %v", src, err)
		}
		if scanned.Reveal() != probe {
			t.Errorf("Scan(%T) produced %q", src, scanned.Reveal())
		}
	}

	var null Secret
	if err = null.Scan(nil); err != nil || !null.Empty() {
		t.Errorf("Scan(nil) = %v, %q", err, null.Reveal())
	}

	// The error names the type it could not scan, never the value.
	var bad Secret
	err = bad.Scan(42)
	if err == nil {
		t.Fatal("Scan accepted a value it cannot hold")
	}
	if strings.Contains(err.Error(), "42") {
		t.Errorf("scan error quoted the value: %v", err)
	}
}
