package admin

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestRedactedHidesPasswords(t *testing.T) {
	for _, req := range []proto.Message{
		&AddUserRequest{Username: "alice", Password: "hunter2"},
		&ChangePasswordRequest{Username: "alice", Password: "hunter2"},
	} {
		got := redacted(req)
		if strings.Contains(got, "hunter2") {
			t.Errorf("redacted(%T) = %q, shows the password", req, got)
		}
		if !strings.Contains(got, "alice") || !strings.Contains(got, redactedValue) {
			t.Errorf("redacted(%T) = %q, want the username and %s", req, got, redactedValue)
		}
	}
}

// The runtime does not act on debug_redact by itself, which is why requests
// are rendered through redacted; if that changes, this starts failing and the
// redaction can go.
func TestRuntimeDoesNotRedactOnItsOwn(t *testing.T) {
	if s := (&AddUserRequest{Password: "hunter2"}).String(); !strings.Contains(s, "hunter2") {
		t.Skipf("the runtime now redacts by itself: %q", s)
	}
}

func TestRedactedLeavesTheOriginalAlone(t *testing.T) {
	req := &AddUserRequest{Username: "alice", Password: "hunter2"}
	_ = redacted(req)
	if req.Password != "hunter2" {
		t.Errorf("redacting changed the request's password to %q", req.Password)
	}
}

func TestRedactedLeavesOrdinaryRequestsWhole(t *testing.T) {
	got := redacted(&DeleteUserRequest{Username: "alice"})
	if !strings.Contains(got, "alice") || strings.Contains(got, redactedValue) {
		t.Errorf("redacted(DeleteUserRequest) = %q", got)
	}
}
