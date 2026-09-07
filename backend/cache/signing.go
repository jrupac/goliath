package cache

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"flag"
	"net/url"

	log "github.com/golang/glog"
)

var imageProxyKey = flag.String("imageProxyKey", "",
	"Secret used to sign image proxy URLs. Required when image proxying is enabled.")

// SignatureParam is the query parameter carrying a proxy URL's signature.
const SignatureParam = "sig"

// ErrNoImageProxyKey reports that image proxying was asked for without a key to
// sign with.
var ErrNoImageProxyKey = errors.New("image proxying is enabled but imageProxyKey is not set")

// CheckImageProxyKey reports whether the proxy is configured well enough to run.
//
// The key must be the same across restarts, unlike a key used only within one
// process lifetime: signatures are written into stored article content when a
// feed is fetched and verified whenever the reader loads an image, possibly
// weeks later. A key generated at startup would invalidate every URL already
// stored, so there is no safe default and one has to be configured.
func CheckImageProxyKey(proxyingEnabled bool) error {
	if proxyingEnabled && *imageProxyKey == "" {
		return ErrNoImageProxyKey
	}
	return nil
}

// SignImageTarget returns the signature for a URL the proxy may fetch.
//
// This is what keeps the endpoint from being an open proxy. It is reachable
// from anywhere and needs to be, since a browser loads an image with an
// ordinary GET, so the restriction cannot come from who is asking. Instead the
// proxy will only fetch a URL that Goliath itself chose to rewrite, and the
// signature is how it recognizes one.
func SignImageTarget(target string) string {
	if *imageProxyKey == "" {
		// Refuses rather than emitting an unsigned URL that the proxy would
		// then have to accept.
		log.Errorf("Cannot sign image proxy URL: no imageProxyKey configured")
		return ""
	}

	mac := hmac.New(sha256.New, []byte(*imageProxyKey))
	mac.Write([]byte(target))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// verifyImageTarget reports whether a signature is one this server produced for
// the given target.
func verifyImageTarget(target, signature string) bool {
	if *imageProxyKey == "" || signature == "" {
		return false
	}

	given, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(*imageProxyKey))
	mac.Write([]byte(target))
	return hmac.Equal(given, mac.Sum(nil))
}

// SignedImageURL returns the proxy URL for a target, or an empty string if it
// cannot be signed.
func SignedImageURL(proxyBase, endpoint, target string) string {
	signature := SignImageTarget(target)
	if signature == "" {
		return ""
	}

	proxyURL, err := url.Parse(proxyBase + "/" + endpoint)
	if err != nil {
		log.Warningf("invalid proxy base URL %s: %s", proxyBase, err)
		return ""
	}

	q := proxyURL.Query()
	q.Set("url", target)
	q.Set(SignatureParam, signature)
	proxyURL.RawQuery = q.Encode()

	return proxyURL.String()
}
