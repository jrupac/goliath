package fetch

import (
	"context"
	"errors"
	"image"
	"net/url"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/utils"
	"github.com/jrupac/rss"
	"github.com/mat/besticon/v3/besticon"
)

func isValidAbsoluteURL(s string) bool {
	if s == "" {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func (f Fetcher) updateFeedMetadataForUser(ctx context.Context, u models.User, mFeed *models.Feed, rFeed *rss.Feed) {
	if rFeed.Title != "" {
		t := maybeUnescapeHtml(rFeed.Title)
		mFeed.Title = extractTextFromHtmlUnsafe(t)
	}
	if rFeed.Description != "" {
		d := maybeUnescapeHtml(rFeed.Description)
		mFeed.Description = extractTextFromHtmlUnsafe(d)
	}
	if rFeed.Link != "" && isValidAbsoluteURL(rFeed.Link) {
		mFeed.Link = rFeed.Link
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
	var feedHost string

	parsedUrl, err := url.Parse(fetch.Link)
	if err == nil {
		feedHost = parsedUrl.Hostname()
	}

	// Look in multiple URLs for a suitable icon
	found := false
	for _, path := range []string{fetch.Image.URL, fetch.Link, feedHost} {
		if i, decoded, err := f.tryIconFetch(path); err == nil {
			found = true
			icon = i
			img = decoded
		}
	}

	if !found {
		log.V(2).Infof("Could not find suitable icon for feed: %s", feedHost)
		return
	}

	ip := maybeResizeImage(feed.FolderID, feed.ID, icon, img)
	utils.DebugPrint("Received a new image:", ip)

	// Check if the context is canceled. If not, updated the favicon.
	if ctx.Err() == nil {
		if err = f.d.InsertFaviconForUser(u, ip.folderId, ip.id, ip.mime, ip.favicon); err != nil {
			log.Warningf("while persisting icon for user %s feed '%s': %s", u, feedHost, err)
		}
	}
}
