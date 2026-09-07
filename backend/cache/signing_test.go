package cache

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

const signingTarget = "http://insecure.example.com/photo.jpg"

func TestSignImageTargetIsDeterministicAndSpecific(t *testing.T) {
	withKey(t)

	first := SignImageTarget(signingTarget)
	if first == "" {
		t.Fatal("SignImageTarget produced nothing with a key configured")
	}

	// Signatures are written into stored article content and verified whenever
	// an image is loaded afterwards, so the same target must always sign the
	// same way.
	if second := SignImageTarget(signingTarget); second != first {
		t.Errorf("SignImageTarget is not stable: %q then %q", first, second)
	}

	if other := SignImageTarget(signingTarget + "?x=1"); other == first {
		t.Error("two different targets signed identically")
	}
}

// A signature is only meaningful if it commits to the target it was issued for.
func TestVerifyImageTargetRejectsMismatches(t *testing.T) {
	withKey(t)

	valid := SignImageTarget(signingTarget)

	if !verifyImageTarget(signingTarget, valid) {
		t.Error("a freshly produced signature did not verify")
	}

	for _, tc := range []struct {
		name      string
		target    string
		signature string
	}{
		{"signature for another target", "http://insecure.example.com/other.jpg", valid},
		{"empty signature", signingTarget, ""},
		{"not base64", signingTarget, "!!!not-base64!!!"},
		{"right length, wrong bytes", signingTarget, SignImageTarget("http://elsewhere.example.com/x.jpg")},
		{"truncated", signingTarget, valid[:len(valid)-2]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if verifyImageTarget(tc.target, tc.signature) {
				t.Error("verified a signature that should not have been accepted")
			}
		})
	}
}

// With no key there is nothing to sign or verify with, and the safe reading of
// that is that nothing is authorized rather than that everything is.
func TestSigningWithoutAKeyAuthorizesNothing(t *testing.T) {
	original := *imageProxyKey
	*imageProxyKey = ""
	t.Cleanup(func() { *imageProxyKey = original })

	if got := SignImageTarget(signingTarget); got != "" {
		t.Errorf("SignImageTarget returned %q without a key", got)
	}
	if verifyImageTarget(signingTarget, "anything") {
		t.Error("verifyImageTarget accepted a signature with no key configured")
	}
	if got := SignedImageURL("https://proxy.example.com", "cache", signingTarget); got != "" {
		t.Errorf("SignedImageURL returned %q without a key", got)
	}
}

// Changing the key invalidates what the previous one signed. This is why the
// key has to stay stable once articles have been stored.
func TestSignaturesDoNotSurviveAKeyChange(t *testing.T) {
	withKey(t)
	before := SignImageTarget(signingTarget)

	*imageProxyKey = "a-different-key"
	if verifyImageTarget(signingTarget, before) {
		t.Error("a signature made with the previous key still verified")
	}
}

func TestSignedImageURLCarriesTargetAndSignature(t *testing.T) {
	withKey(t)

	signed := SignedImageURL("https://proxy.example.com", "cache", signingTarget)
	if signed == "" {
		t.Fatal("SignedImageURL produced nothing")
	}
	if !strings.HasPrefix(signed, "https://proxy.example.com/cache?") {
		t.Errorf("SignedImageURL = %q, want it rooted at the proxy base", signed)
	}

	parsed, err := url.Parse(signed)
	if err != nil {
		t.Fatalf("parsing %q: %v", signed, err)
	}
	if got := parsed.Query().Get("url"); got != signingTarget {
		t.Errorf("url = %q, want %q", got, signingTarget)
	}
	if !verifyImageTarget(signingTarget, parsed.Query().Get(SignatureParam)) {
		t.Error("the URL it produced does not carry a signature this server accepts")
	}
}

// Proxying without a key would mean a proxy that fetches whatever it is handed,
// so the configuration is refused rather than run.
func TestCheckImageProxyKey(t *testing.T) {
	original := *imageProxyKey
	t.Cleanup(func() { *imageProxyKey = original })

	*imageProxyKey = ""
	if err := CheckImageProxyKey(false); err != nil {
		t.Errorf("proxying disabled and no key should be fine, got %v", err)
	}
	if err := CheckImageProxyKey(true); !errors.Is(err, ErrNoImageProxyKey) {
		t.Errorf("CheckImageProxyKey(true) = %v, want %v", err, ErrNoImageProxyKey)
	}

	*imageProxyKey = "configured"
	if err := CheckImageProxyKey(true); err != nil {
		t.Errorf("CheckImageProxyKey(true) with a key = %v, want nil", err)
	}
}
