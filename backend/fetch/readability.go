package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/PuerkitoBio/goquery"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/utils"
)

var (
	fullTextTimeout   = flag.Duration("fullTextTimeout", 10*time.Second, "Timeout for full-text extraction requests.")
	fullTextUserAgent = flag.String("fullTextUserAgent", "",
		"Overrides userAgent for full-text extraction only, for a site that serves different content by client. Empty means use userAgent.")
)

var (
	ErrEmptyContent = errors.New("extracted readability content was empty")
)

var (
	extractorOnce sync.Once
	extractor     *articleExtractor
)

// articleExtractor fetches articles and extracts their full text.
type articleExtractor struct {
	client    *http.Client
	userAgent string
}

// newArticleExtractor returns an extractor whose requests are address-guarded,
// reaching addresses on allowed anyway.
//
// An article's link comes from its feed, so it is a URL this process did not
// choose. Redirects are followed to any host, since articles routinely move
// between them: each hop is dialed through the same guard, so following one
// reaches nowhere the link itself could not.
func newArticleExtractor(timeout time.Duration, userAgent string, allowed utils.AddressAllowlist) *articleExtractor {
	return &articleExtractor{
		client: &http.Client{
			Timeout:   timeout,
			Transport: utils.GuardedTransport("Full-text extraction", timeout, allowed),
		},
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

	content = promoteImageSources(content)
	content = rewriteFragmentUrls(content, parsedURL)

	return content, nil
}

// ExtractFullText initializes/retrieves the singleton extractor and performs the extraction.
func ExtractFullText(ctx context.Context, url string) (string, error) {
	extractorOnce.Do(func() {
		timeout := 10 * time.Second
		if fullTextTimeout != nil && *fullTextTimeout > 0 {
			timeout = *fullTextTimeout
		}
		userAgent := UserAgent()
		if *fullTextUserAgent != "" {
			userAgent = *fullTextUserAgent
		}
		log.Infof("Initializing full-text article extractor (timeout=%s, userAgent=%s)", timeout, userAgent)
		extractor = newArticleExtractor(timeout, userAgent, nil)
	})

	return extractor.Extract(ctx, url)
}

func parseSrcset(srcset string) string {
	parts := strings.Split(srcset, ",")
	var bestURL string
	var maxVal float64 = -1

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		subparts := strings.Fields(part)
		if len(subparts) == 0 {
			continue
		}
		imgURL := subparts[0]
		if len(subparts) == 1 {
			if bestURL == "" {
				bestURL = imgURL
			}
			continue
		}
		desc := subparts[1]
		var val float64
		var err error
		if strings.HasSuffix(desc, "w") {
			val, err = strconv.ParseFloat(strings.TrimSuffix(desc, "w"), 64)
		} else if strings.HasSuffix(desc, "x") {
			val, err = strconv.ParseFloat(strings.TrimSuffix(desc, "x"), 64)
		}
		if err == nil && val > maxVal {
			maxVal = val
			bestURL = imgURL
		} else if bestURL == "" {
			bestURL = imgURL
		}
	}
	return bestURL
}

type dataLoading struct {
	Desktop string `json:"desktop"`
	Mobile  string `json:"mobile"`
}

func parseDataLoading(jsonStr string) string {
	var dl dataLoading
	if err := json.Unmarshal([]byte(jsonStr), &dl); err == nil {
		if dl.Desktop != "" {
			return dl.Desktop
		}
		if dl.Mobile != "" {
			return dl.Mobile
		}
	}
	return ""
}

func promoteImageSources(content string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		log.Warningf("failed to parse HTML for image source promotion: %s", err)
		return content
	}

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		// Check data-src / data-original (standard lazy loading)
		if val, exists := s.Attr("data-src"); exists && val != "" {
			s.SetAttr("src", val)
			return
		}
		if val, exists := s.Attr("data-original"); exists && val != "" {
			s.SetAttr("src", val)
			return
		}

		// Check data-loading (custom JSON attribute, e.g., Google Blog)
		if val, exists := s.Attr("data-loading"); exists && val != "" {
			if dlURL := parseDataLoading(val); dlURL != "" {
				s.SetAttr("src", dlURL)
				return
			}
		}

		// Check srcset
		if val, exists := s.Attr("srcset"); exists && val != "" {
			if highRes := parseSrcset(val); highRes != "" {
				s.SetAttr("src", highRes)
			}
		}
	})

	htmlStr, err := doc.Html()
	if err != nil {
		log.Warningf("failed to render HTML after image source promotion: %s", err)
		return content
	}
	return htmlStr
}

func rewriteFragmentUrls(content string, articleURL *url.URL) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		log.Warningf("failed to parse HTML for fragment URL rewriting: %s", err)
		return content
	}

	normalizePath := func(p string) string {
		return strings.TrimSuffix(p, "/")
	}

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}
		parsedHref, err := url.Parse(href)
		if err != nil {
			return
		}
		if parsedHref.Scheme == articleURL.Scheme &&
			parsedHref.Host == articleURL.Host &&
			normalizePath(parsedHref.Path) == normalizePath(articleURL.Path) &&
			parsedHref.Fragment != "" {
			s.SetAttr("href", "#"+parsedHref.Fragment)
		}
	})

	htmlStr, err := doc.Html()
	if err != nil {
		log.Warningf("failed to render HTML after fragment URL rewriting: %s", err)
		return content
	}
	return htmlStr
}

// SetClientForTesting overrides the HTTP client of the default extractor for unit testing.
func SetClientForTesting(client *http.Client) {
	extractorOnce.Do(func() {
		extractor = newArticleExtractor(10*time.Second, "Goliath/Test", nil)
	})
	extractor.client = client
}
