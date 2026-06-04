package fetch

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"syscall"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	log "github.com/golang/glog"
)

var (
	fullTextTimeout   = flag.Duration("fullTextTimeout", 10*time.Second, "Timeout for full-text extraction requests.")
	fullTextUserAgent = flag.String("fullTextUserAgent", "Goliath/1.0 (+http://github.com/jrupac/goliath)", "User-Agent header sent during full-text extraction.")
)

var (
	ErrEmptyContent = errors.New("extracted readability content was empty")
)

var (
	extractorOnce sync.Once
	extractor     *articleExtractor
)

// articleExtractor handles secure fetching and parsing of full article text.
type articleExtractor struct {
	client    *http.Client
	userAgent string
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsPrivate() {
		return true
	}
	// CGNAT range: 100.64.0.0/10
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && (ip4[1] >= 64 && ip4[1] <= 127) {
			return true
		}
	}
	return false
}

// newArticleExtractor creates a hardened HTTP client with SSRF protection and timeout.
func newArticleExtractor(timeout time.Duration, userAgent string) *articleExtractor {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip != nil && isPrivateIP(ip) {
				return fmt.Errorf("SSRF protection: access to private IP %s is blocked", ip)
			}
			return nil
		},
	}

	// Double check IP resolution during network dial
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			// Block cross-host redirects to avoid SSRF bypass via redirect to metadata endpoints
			if len(via) > 0 {
				if req.URL.Host != via[0].URL.Host {
					return fmt.Errorf("SSRF protection: cross-host redirect from %s to %s is blocked", via[0].URL.Host, req.URL.Host)
				}
			}
			return nil
		},
	}

	return &articleExtractor{
		client:    client,
		userAgent: userAgent,
	}
}

func (e *articleExtractor) Extract(ctx context.Context, articleURL string) (string, error) {
	parsedURL, err := url.Parse(articleURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", articleURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", e.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("non-200 HTTP status: %d", resp.StatusCode)
	}

	art, err := readability.FromReader(resp.Body, parsedURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse readability content: %w", err)
	}

	var buf bytes.Buffer
	if err := art.RenderHTML(&buf); err != nil {
		return "", fmt.Errorf("failed to render HTML: %w", err)
	}

	content := buf.String()
	if content == "" {
		return "", ErrEmptyContent
	}

	return content, nil
}

// ExtractFullText initializes/retrieves the singleton extractor and performs the extraction.
func ExtractFullText(ctx context.Context, url string) (string, error) {
	extractorOnce.Do(func() {
		timeout := 10 * time.Second
		if fullTextTimeout != nil && *fullTextTimeout > 0 {
			timeout = *fullTextTimeout
		}
		userAgent := "Goliath/1.0 (+http://github.com/jrupac/goliath)"
		if fullTextUserAgent != nil && *fullTextUserAgent != "" {
			userAgent = *fullTextUserAgent
		}
		log.Infof("Initializing full-text article extractor (timeout=%s, userAgent=%s)", timeout, userAgent)
		extractor = newArticleExtractor(timeout, userAgent)
	})

	return extractor.Extract(ctx, url)
}
