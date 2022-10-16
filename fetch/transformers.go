package fetch

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/disintegration/imaging"
	log "github.com/golang/glog"
	"github.com/mat/besticon/besticon"
	"html"
	"image"
	"image/png"
	"net/url"
	"strings"
)

const (
	cacheEndpoint = "cache"
)

var (
	proxyInsecureImages = flag.Bool("proxyInsecureImages", false, "If true, image 'src' attributes are rewritten to be reverse proxied over HTTPS.")
	proxySecureImages   = flag.Bool("proxySecureImages", false, "If true, also rewritten images served over HTTPS to a proxy server.")
	proxyUrlBase        = flag.String("proxyUrlBase", "", "Base URL to reverse image proxy server.")
)

// maybeResizeImage converts the provided besticon.Icon to a 256x256 PNG image
// and returns an imagePair struct containing the base64-encoded image and
// metadata.
func maybeResizeImage(folderId int64, feedId int64, bi besticon.Icon, i *image.Image) (ip imagePair) {
	ip = imagePair{folderId, feedId, "image/" + bi.Format, bi.ImageData}

	if *normalizeFavicons {
		var buff bytes.Buffer

		resized := imaging.Resize(*i, 256, 256, imaging.Lanczos)

		err := png.Encode(&buff, resized)
		if err != nil {
			log.Warningf("failed to encode image as PNG: %s", err)
			return
		}

		ip = imagePair{folderId, feedId, "image/png", buff.Bytes()}
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

// maybeRewriteImageSourceUrls parses the given string as HTML, searches for
// image source URLs and then rewrites them to point at the reverse image proxy.
func maybeRewriteImageSourceUrls(s string) string {
	// If we get an empty string, don't try to parse it. Doing so and then
	// re-rendering will produce a non-empty but semantically empty HTML document.
	if s == "" {
		return s
	}

	if !*proxyInsecureImages {
		return s
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(s))
	if err != nil {
		log.Warningf("Failed to parse HTML: %s", err)
		return s
	}

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		for _, attr := range s.Nodes[0].Attr {
			if attr.Key == "src" {

				imgUrl, err := url.Parse(attr.Val)
				if err != nil {
					log.Warningf("Could not parse img src %s: %s", attr.Val, err)
				}

				if imgUrl.Scheme == "https" && !*proxySecureImages {
					continue
				}

				newUrl, err := url.Parse(fmt.Sprintf("%s/%s", *proxyUrlBase, cacheEndpoint))
				if err != nil {
					log.Warningf("Invalid proxy base URL %s: %s", *proxyUrlBase, err)
					break
				}

				q := newUrl.Query()
				q.Add("url", attr.Val)
				newUrl.RawQuery = q.Encode()

				log.V(2).Infof("Rewritten URL: %s", newUrl.String())

				s.SetAttr(attr.Key, newUrl.String())
			}
		}
	})

	resp, err := doc.Html()
	if err != nil {
		log.Warningf("Failed to render rewritten HTML: %s", err)
		return s
	}

	return resp
}
