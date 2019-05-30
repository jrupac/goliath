package fetch

import (
	"bytes"
	"flag"
	"github.com/PuerkitoBio/goquery"
	"github.com/disintegration/imaging"
	"github.com/golang/glog"
	"github.com/mat/besticon/besticon"
	"html"
	"image/png"
	"net/url"
	"strings"
)

var (
	rewriteInsecureImageUrls = flag.Bool("rewriteInsecureImageUrls", false, "If true, image 'src' attributes are rewritten to be reverse proxied over HTTPS.")
	rewriteSecureImageUrls   = flag.Bool("rewriteSecureImageUrls", false, "If true, also rewritten images served over HTTPS to a proxy server.")
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

// maybeRewriteImageSourceUrls parses the given string as HTML, searches for
// image source URLs and then rewrites them to point at the reverse image proxy.
func maybeRewriteImageSourceUrls(s string) string {
	if !*rewriteInsecureImageUrls {
		return s
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(s))
	if err != nil {
		glog.Warningf("Failed to parse HTML: %s", err)
		return s
	}

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		for _, attr := range s.Nodes[0].Attr {
			if attr.Key == "src" {

				imgUrl, err := url.Parse(attr.Val)
				if err != nil {
					glog.Warningf("Could not parse img src %s: %s", attr.Val, err)
				}

				if imgUrl.Scheme == "https" && !*rewriteSecureImageUrls {
					continue
				}

				newUrl := url.URL{
					Path: "/cache",
				}
				q := newUrl.Query()
				q.Add("url", attr.Val)
				newUrl.RawQuery = q.Encode()

				glog.V(2).Infof("Rewritten URL: %s", newUrl.String())

				s.SetAttr(attr.Key, newUrl.String())
			}
		}
	})

	resp, err := doc.Html()
	if err != nil {
		glog.Warningf("Failed to render rewritten HTML: %s", err)
		return s
	}

	return resp
}
