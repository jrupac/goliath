package fetch

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/storage"
)

// ShrinkStoredFavicons re-encodes every stored favicon that is not already a
// PNG of the size favicons are normalized to.
//
// Favicons are normalized as they are fetched, but a feed's favicon is fetched
// again only when the feed serves a new document, so one stored at another
// size can otherwise stay that way indefinitely. Once every stored favicon has
// been through this, it finds nothing to do.
func ShrinkStoredFavicons(ctx context.Context, d storage.Database) {
	if !*normalizeFavicons {
		return
	}

	users, err := d.GetAllUsers()
	if err != nil {
		log.Warningf("Could not list users to shrink their favicons: %s", err)
		return
	}

	shrunk := 0
	for _, u := range users {
		if ctx.Err() != nil {
			return
		}
		favicons, err := d.GetAllFaviconsForUser(u)
		if err != nil {
			log.Warningf("Could not read favicons for user %s: %s", u, err)
			continue
		}
		for feedId, stored := range favicons {
			// Stored as "<mime>;base64,<data>".
			_, encoded, ok := strings.Cut(stored, ";base64,")
			if !ok {
				continue
			}
			raw, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				log.Warningf("Unreadable favicon for user %s feed %d: %s", u, feedId, err)
				continue
			}
			cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
			if err != nil {
				log.Warningf("Undecodable favicon for user %s feed %d: %s", u, feedId, err)
				continue
			}
			if format == "png" && cfg.Width == faviconSize && cfg.Height == faviconSize {
				continue
			}
			img, _, err := image.Decode(bytes.NewReader(raw))
			if err != nil {
				log.Warningf("Undecodable favicon for user %s feed %d: %s", u, feedId, err)
				continue
			}
			normalized, err := normalizeFavicon(img)
			if err != nil {
				log.Warningf("Could not re-encode favicon for user %s feed %d: %s", u, feedId, err)
				continue
			}
			if err = d.InsertFaviconForUser(u, feedId, "image/png", normalized); err != nil {
				log.Warningf("Could not store favicon for user %s feed %d: %s", u, feedId, err)
				continue
			}
			shrunk++
		}
	}
	if shrunk > 0 {
		log.Infof("Re-encoded %d stored favicons at %dx%d.", shrunk, faviconSize, faviconSize)
	}
}
