package fetch

import (
	"bytes"
	"github.com/disintegration/imaging"
	"github.com/golang/glog"
	"github.com/mat/besticon/besticon"
	"html"
	"image/png"
	"strings"
)

// maybeResizeImage converts the provided besticon.Icon to a 256x256 PNG image
// and returns an imagePair struct containing the base64-encoded image and
// metadata.
func maybeResizeImage(feedId int64, bi besticon.Icon) (ip imagePair) {
	ip = imagePair{feedId, "image/" + bi.Format, bi.ImageData}

	if *normalizeFavicons {
		var buff bytes.Buffer

		i, err := bi.Image()
		if err != nil {
			glog.Warningf("failed to convert besticon.Icon to image.Image: %s", err)
			return
		}

		resized := imaging.Resize(*i, 256, 256, imaging.Lanczos)

		err = png.Encode(&buff, resized)
		if err != nil {
			glog.Warningf("failed to encode image as PNG: %s", err)
			return
		}

		ip = imagePair{feedId, "image/png", buff.Bytes()}
	}

	return
}

// maybeUnescapeHtml looks for occurrences of escaped HTML characters. If more
// than one is found in the given string, an HTML-unescaped string is returned.
// Otherwise, the given input is unmodified.
func maybeUnescapeHtml(content string) string {
	occLimit := 1
	occ := 0
	// The HTML standard defines escape sequences for &, <, and >.
	escapes := []string{"&amp;", "&lt;", "&gt;", "&#34;", "&apos;"}

	for _, seq := range escapes {
		occ += strings.Count(content, seq)
	}

	if occ > occLimit {
		return html.UnescapeString(content)
	}
	return content
}
