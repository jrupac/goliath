package cache

import (
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/utils"
)

const (
	// maxImageBytes bounds what will be relayed for a single image. The target
	// comes from feed content, so without a bound a feed could name a file
	// large enough to exhaust memory just by being linked from an article.
	maxImageBytes = 16 << 20

	// proxyTimeout bounds a single fetch.
	proxyTimeout = 5 * time.Second
)

// forwardedResponseHeaders are relayed from the origin to the browser.
// The client is responsible for handling caching, not this proxy.
var forwardedResponseHeaders = []string{
	"Content-Type",
	"Cache-Control",
	"ETag",
	"Expires",
	"Last-Modified",
	"Age",
	"Vary",
}

// forwardedRequestHeaders are relayed from the browser to the origin, so that a
// browser holding a cached copy can revalidate it rather than refetch it.
var forwardedRequestHeaders = []string{
	"If-None-Match",
	"If-Modified-Since",
	"Accept",
}

type imageProxy struct {
	Client *http.Client
}

// NewImageProxy returns an HTTP handler that relays images named by signed
// URLs.
//
// Nothing is cached here. The targets are supplied by feed publishers, so
// storing what comes back would mean holding arbitrary third-party content;
// the browser caches instead, according to the origin's own headers.
func NewImageProxy() http.Handler {
	return &imageProxy{
		Client: &http.Client{
			Timeout:   proxyTimeout,
			Transport: utils.GuardedTransport("Image proxy", proxyTimeout),
			// Redirects are followed, since image hosts and CDNs use them
			// routinely, but each hop is dialed through the same guard, so a
			// redirect cannot reach anywhere the original URL could not.
		},
	}
}

// DenyUnauthenticated refuses a request that arrives without a session.
//
// It deliberately does not fall back to redirecting the browser to the image's
// original URL. That fallback sent the browser to an insecure origin, which is
// the outcome proxying exists to avoid, and it did so silently -- a page that
// looks fine while having quietly downgraded. Failing outright makes a broken
// image the visible symptom of a real problem.
func DenyUnauthenticated(w http.ResponseWriter, r *http.Request) {
	utils.HttpRequestPrint("Received unauthenticated image proxy request", r)
	w.WriteHeader(http.StatusForbidden)
}

func (p *imageProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target, ok := p.resolveTarget(w, r)
	if !ok {
		return
	}

	log.V(2).Infof("Proxying request to: %s", target)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for _, h := range forwardedRequestHeaders {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		log.Warningf("Failed to proxy request: %s", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warningf("Failed to close proxied response body: %s", err)
		}
	}()

	// Relayed as-is so a browser holding a cached copy learns it is still good
	// without the image being sent again.
	if resp.StatusCode == http.StatusNotModified {
		copyHeaders(w, resp)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Warningf("Proxy target returned non-200 status: %d", resp.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	// Only images can be proxied through this server.
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "image/") {
		log.Warningf("Proxy target returned non-image content type: %q", contentType)
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	copyHeaders(w, resp)
	w.WriteHeader(http.StatusOK)

	// Streamed rather than buffered, and bounded. A response longer than the
	// bound is truncated rather than rejected, since by this point the status
	// has been sent; the browser sees a broken image, which is the same outcome
	// as refusing it.
	if _, err = io.Copy(w, io.LimitReader(resp.Body, maxImageBytes)); err != nil {
		log.Warningf("Could not write proxied response back to client: %s", err)
	}
}

// resolveTarget returns the URL to fetch, writing the error response and
// reporting false if the request does not name one this server authorized.
func (p *imageProxy) resolveTarget(w http.ResponseWriter, r *http.Request) (string, bool) {
	val := r.URL.Query().Get("url")
	if val == "" {
		w.WriteHeader(http.StatusBadRequest)
		return "", false
	}
	target := html.UnescapeString(val)

	// Checked before anything else is done with the URL: an unsigned request
	// names a target this server never chose, and the only thing to do with it
	// is refuse.
	if !verifyImageTarget(target, r.URL.Query().Get(SignatureParam)) {
		log.Warningf("Image proxy refused an unsigned or mis-signed request")
		w.WriteHeader(http.StatusForbidden)
		return "", false
	}

	parsed, err := url.Parse(target)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		log.Warningf("Image proxy refused scheme in target URL")
		w.WriteHeader(http.StatusBadRequest)
		return "", false
	}

	return target, true
}

func copyHeaders(w http.ResponseWriter, resp *http.Response) {
	for _, h := range forwardedResponseHeaders {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
}
