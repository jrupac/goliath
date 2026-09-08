package utils

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Targets come from feed publishers, so a feed must not be able to make this
// server reach something only it can reach.
func TestIsPublicAddressRefusesInternalRanges(t *testing.T) {
	for _, addr := range []string{
		"127.0.0.1", "::1", // loopback
		"10.0.0.1", "192.168.1.1", "172.16.0.1", // private
		"fd00::1",         // unique local
		"169.254.169.254", // link-local, where cloud metadata lives
		"fe80::1",
		"0.0.0.0", "::",
		"224.0.0.1", "ff02::1", // multicast
	} {
		if IsPublicAddress(net.ParseIP(addr)) {
			t.Errorf("IsPublicAddress(%s) = true, want false", addr)
		}
	}

	for _, addr := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !IsPublicAddress(net.ParseIP(addr)) {
			t.Errorf("IsPublicAddress(%s) = false, want true", addr)
		}
	}
}

// The guard is installed on the transport rather than applied to the URL, so it
// runs for every connection the client makes. That is what covers redirects and
// a name that resolves differently the second time it is looked up: both go
// through a fresh dial, and each one is checked.
func TestGuardedTransportRefusesInternalDials(t *testing.T) {
	client := &http.Client{Transport: GuardedTransport("Test", 5*time.Second, nil)}

	for _, target := range []string{
		"http://127.0.0.1:80/",
		"http://10.0.0.1:80/",
		"http://169.254.169.254:80/latest/meta-data/",
		"http://[::1]:80/",
	} {
		resp, err := client.Get(target)
		if err == nil {
			_ = resp.Body.Close()
			t.Errorf("dialed %s, which should have been refused", target)
			continue
		}
		if !strings.Contains(err.Error(), ErrBlockedAddress.Error()) {
			t.Errorf("%s failed with %v, want it refused by the address guard", target, err)
		}
	}
}

// Images are served over the ordinary web ports. Anything else is a service
// that happens to speak HTTP, which is not what this is for.
func TestGuardedTransportRefusesOtherPorts(t *testing.T) {
	client := &http.Client{Transport: GuardedTransport("Test", 5*time.Second, nil)}

	resp, err := client.Get("http://8.8.8.8:8080/")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("dialed a non-web port, which should have been refused")
	}
	if !strings.Contains(err.Error(), ErrBlockedAddress.Error()) {
		t.Errorf("failed with %v, want it refused by the address guard", err)
	}
}

// The allowlist exists so an operator can run a feed bridge alongside this
// server. It is compared against the address dialed, so a name in the
// configuration has to be resolved to get there.
func TestAddressAllowlistPermitsConfiguredAddress(t *testing.T) {
	allowed, err := NewAddressAllowlist([]string{"127.0.0.1:1200"})
	if err != nil {
		t.Fatalf("NewAddressAllowlist: %v", err)
	}
	if !allowed.Permits("127.0.0.1:1200") {
		t.Error("the configured address is not permitted")
	}
	// Neither a different port on the same host nor the same port on a
	// different host was configured.
	for _, addr := range []string{"127.0.0.1:1201", "10.0.0.1:1200", "169.254.169.254:1200"} {
		if allowed.Permits(addr) {
			t.Errorf("%s is permitted but was not configured", addr)
		}
	}
}

// A name is resolved when the allowlist is built, so that the guard can compare
// against the address actually being dialed.
func TestAddressAllowlistResolvesNames(t *testing.T) {
	allowed, err := NewAddressAllowlist([]string{"localhost:1200"})
	if err != nil {
		t.Fatalf("NewAddressAllowlist: %v", err)
	}
	if !allowed.Permits("127.0.0.1:1200") && !allowed.Permits("[::1]:1200") {
		t.Errorf("localhost:1200 resolved to none of the loopback addresses: %v", allowed)
	}
}

// An allowlist that cannot be understood is a configuration error, not an
// empty allowlist: silently allowing nothing would refuse the feeds it was
// written to permit.
func TestAddressAllowlistRejectsUnusableEntries(t *testing.T) {
	for _, entry := range []string{"127.0.0.1", "no-port-here", "host.invalid:1200"} {
		if _, err := NewAddressAllowlist([]string{entry}); err == nil {
			t.Errorf("NewAddressAllowlist(%q) succeeded, want an error", entry)
		}
	}

	// Blank entries are the ordinary result of splitting an empty or
	// trailing-comma setting, and mean no allowlist rather than a bad one.
	allowed, err := NewAddressAllowlist([]string{"", "  "})
	if err != nil {
		t.Errorf("NewAddressAllowlist on blank entries: %v", err)
	}
	if len(allowed) != 0 {
		t.Errorf("blank entries produced %v", allowed)
	}
}

// The allowlist is consulted before the address and port rules, so a permitted
// address is reached on a port the guard would otherwise refuse.
func TestGuardedTransportReachesAllowlistedAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("feed"))
	}))
	defer server.Close()

	// The test server listens on loopback and an arbitrary high port, which is
	// refused on both counts without an allowlist.
	addr := strings.TrimPrefix(server.URL, "http://")
	blocked := &http.Client{Transport: GuardedTransport("Test", 5*time.Second, nil)}
	if resp, err := blocked.Get(server.URL); err == nil {
		_ = resp.Body.Close()
		t.Fatal("reached the server without an allowlist")
	}

	allowed, err := NewAddressAllowlist([]string{addr})
	if err != nil {
		t.Fatalf("NewAddressAllowlist: %v", err)
	}
	client := &http.Client{Transport: GuardedTransport("Test", 5*time.Second, allowed)}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("allowlisted address was refused: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "feed" {
		t.Errorf("body = %q, want %q", body, "feed")
	}
}
