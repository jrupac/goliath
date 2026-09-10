package fetch

import (
	"context"
	"errors"
	"image"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/utils"
	"github.com/jrupac/rss"
	"github.com/mat/besticon/v3/besticon"
)

// isValidAbsoluteURL reports whether a URL taken from feed markup is usable as
// an absolute address.
//
// Trimmed first, because an element written across several lines carries the
// surrounding whitespace into its text, and a URL parser rejects the newlines
// as control characters. The address itself is fine; only its presentation in
// the document was not.
func isValidAbsoluteURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func (f Fetcher) updateFeedMetadataForUser(ctx context.Context, u models.User, mFeed *models.Feed, rFeed *rss.Feed) {
	// A title the user set is theirs to keep. This runs on every first fetch,
	// and pausing the fetcher to add or remove a subscription makes the next
	// fetch a first fetch, so without this a rename survives only until the
	// next change to the feed list.
	if rFeed.Title != "" && !mFeed.TitleOverridden {
		// Feed titles are plain text (possibly with HTML entities), not HTML
		// documents. Use html.UnescapeString to decode entities without parsing
		// angle brackets as tags — this preserves titles like "<antirez>".
		mFeed.Title = strings.TrimSpace(html.UnescapeString(rFeed.Title))
	}
	if rFeed.Description != "" {
		mFeed.Description = strings.TrimSpace(html.UnescapeString(rFeed.Description))
	}
	if isValidAbsoluteURL(rFeed.Link) {
		mFeed.Link = strings.TrimSpace(rFeed.Link)
	}

	// Check if the context is canceled. If not, updated the feed metadata.
	if ctx.Err() == nil {
		err := f.d.UpdateFeedMetadataForUser(u, *mFeed)
		if err != nil {
			log.Warningf("while updating metadata for user %s feed '%s': %s", u, mFeed.URL, err)
		}
	}
}

func (f Fetcher) tryIconFetch(link string) (besticon.Icon, *image.Image, error) {
	icon := besticon.Icon{}

	if link == "" {
		return icon, nil, errors.New("invalid URL")
	}

	icons, err := f.finder.FetchIcons(link)
	if err != nil {
		return icon, nil, err
	}

	if len(icons) == 0 {
		return icon, nil, errors.New("no icons found")
	}

	for _, i := range icons {
		if i.URL != "" && i.Format != "" {
			// Also try decoding the image. If we're successful, return it to avoid
			// needing to decode it again later on.
			if img, err := i.Image(); err == nil {
				return i, img, nil
			}
		}
	}

	return icon, nil, errors.New("no suitable icons found")
}

func (f Fetcher) updateFeedFaviconForUser(ctx context.Context, u models.User, feed *models.Feed, fetch *rss.Feed) {
	var icon besticon.Icon
	var img *image.Image
	var fetchHost, feedHost string

	parsedUrl, err := url.Parse(fetch.Link)
	if err == nil {
		fetchHost = parsedUrl.Hostname()
	}

	parsedUrl, err = url.Parse(feed.Link)
	if err == nil {
		feedHost = parsedUrl.Hostname()
	}

	// Look in multiple URLs for a suitable icon
	found := false
	for _, path := range []string{fetch.Image.URL, fetch.Link, fetchHost, feedHost} {
		if i, decoded, err := f.tryIconFetch(path); err == nil {
			found = true
			icon = i
			img = decoded
		}
	}

	if !found {
		log.V(2).Infof("Could not find suitable icon for feed: %s", fetchHost)
		return
	}

	ip := maybeResizeImage(feed.FolderID, feed.ID, icon, img)
	utils.DebugPrint("Received a new image:", ip)

	// Check if the context is canceled. If not, updated the favicon.
	if ctx.Err() == nil {
		if err = f.d.InsertFaviconForUser(u, ip.folderId, ip.id, ip.mime, ip.favicon); err != nil {
			log.Warningf("while persisting icon for user %s feed '%s': %s", u, fetchHost, err)
		}
	}
}
