package cache

import (
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/utils"
	"html"
	"io/ioutil"
	"net/http"
	"time"
)

type imageProxy struct {
	client http.Client
}

// NewImageProxy returns an HTTP handler that serves as a reverse image
// proxy for the given request. Cookie verification is already handled by the
// time the request arrives here.
func NewImageProxy() http.Handler {
	return &imageProxy{
		client: http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// AuthErrorRedirect redirects the user to the original proxied URL with a HTTP
// status of 301 ("Moved Permanently").
func AuthErrorRedirect(w http.ResponseWriter, r *http.Request) {
	utils.HttpRequestPrint("Received unauthenticated request", r)

	val := r.URL.Query().Get("url")
	if val == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, html.UnescapeString(val), http.StatusMovedPermanently)
}

func (p *imageProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	val := r.URL.Query().Get("url")
	if val == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	target := html.UnescapeString(val)

	// TODO: Check cache here.

	log.V(2).Infof("Proxying request to: %s", target)

	resp, err := p.client.Get(target)
	if err != nil {
		log.Warningf("Failed to proxy request: %s", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	// TODO: Write to cache here.

	if resp == nil {
		log.Fatalf("Response should not be nil when there is not error.")
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Warningf("Could not read proxied response: %s", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	_, err = w.Write(b)
	if err != nil {
		log.Warningf("Could not write proxied response back to client: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
