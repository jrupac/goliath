package fetch

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/utils"
	"github.com/jrupac/rss"
)

// discoverTimeout bounds the check made before a subscription is created. It
// is shorter than a background fetch: someone is waiting on the answer, and a
// host too slow to respond in this long is better reported than waited for.
const discoverTimeout = 8 * time.Second

// discoverClient fetches a URL that is being considered as a subscription.
//
// Address-guarded with no allowlist, unlike the client that fetches feeds
// already subscribed to. This one is handed a URL by whoever is asking, so it
// is the boundary the guard exists for; the allowlist is for feeds an operator
// has already accepted.
var discoverClient = &http.Client{
	Timeout:   discoverTimeout,
	Transport: utils.GuardedTransport("Feed discovery", discoverTimeout, nil),
}

// DiscoverFeed fetches `feedURL` and returns what is known about it, or an
// error describing why it cannot be subscribed to.
//
// Fetching before storing is the point: a URL that is not a feed becomes a
// subscription that fails forever otherwise, and nothing about it would say
// why. It also supplies the title and site link, which a client adding a feed
// by URL alone cannot.
func DiscoverFeed(feedURL string) (models.Feed, error) {
	feed := models.Feed{URL: feedURL}

	parsed, err := url.Parse(feedURL)
	if err != nil {
		return feed, fmt.Errorf("not a URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return feed, fmt.Errorf("scheme %q is not fetchable", parsed.Scheme)
	}
	if parsed.Host == "" {
		return feed, fmt.Errorf("no host in %q", feedURL)
	}

	fetched, err := rss.FetchByFunc(fetchFuncWithClient(discoverClient), feedURL)
	if err != nil {
		return feed, fmt.Errorf("could not read a feed at %s: %w", feedURL, err)
	}

	// Titles are plain text that may carry entities, not HTML documents, so
	// they are unescaped rather than parsed.
	feed.Title = strings.TrimSpace(html.UnescapeString(fetched.Title))
	feed.Description = strings.TrimSpace(html.UnescapeString(fetched.Description))
	if isValidAbsoluteURL(fetched.Link) {
		feed.Link = strings.TrimSpace(fetched.Link)
	}

	// A feed that names no title of its own still has to be identifiable in a
	// list, and its own address is the only thing left to call it.
	if feed.Title == "" {
		feed.Title = feedURL
	}
	// The site link is what a reader opens when it wants the feed's home. A
	// feed that declares none leaves its own origin as the closest thing to
	// one, which is better than sending a reader to the XML.
	if feed.Link == "" {
		feed.Link = parsed.Scheme + "://" + parsed.Host
	}

	return feed, nil
}
