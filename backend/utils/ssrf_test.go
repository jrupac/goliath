package utils

import (
	"net"
	"net/http"
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
	client := &http.Client{Transport: GuardedTransport("Test", 5*time.Second)}

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
	client := &http.Client{Transport: GuardedTransport("Test", 5*time.Second)}

	resp, err := client.Get("http://8.8.8.8:8080/")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("dialed a non-web port, which should have been refused")
	}
	if !strings.Contains(err.Error(), ErrBlockedAddress.Error()) {
		t.Errorf("failed with %v, want it refused by the address guard", err)
	}
}
