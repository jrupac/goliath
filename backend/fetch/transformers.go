package fetch

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/arbovm/levenshtein"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/rss"
	"github.com/kljensen/snowball"
	"github.com/mat/besticon/v3/besticon"
	"golang.org/x/image/draw"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"image"
	"image/png"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	cacheEndpoint = "cache"
)

var (
	proxyInsecureImages = flag.Bool("proxyInsecureImages", false, "If true, image 'src' attributes are rewritten to be reverse proxied over HTTPS.")
	proxySecureImages   = flag.Bool("proxySecureImages", false, "If true, also rewritten images served over HTTPS to a proxy server.")
	proxyUrlBase        = flag.String("proxyUrlBase", "", "Base URL to reverse image proxy server.")
)

// processItem creates a models.Article object from an RSS item, applying
// transformations and sanitizations as needed.
func processItem(feed *models.Feed, item *rss.Item) models.Article {
	title := item.Title
	// An empty title can cause rendering problems and just looks wrong when
	// being displayed, so rewrite.
	if title == "" {
		title = "(Untitled)"
	}

	// Try both item.Content and item.Summary to see if they have any values
	contents := item.Content
	if contents == "" {
		contents = item.Summary
	}

	// Some feeds give back content that is HTML-escaped. When this happens,
	// sanitization makes the content appear as raw, escaped text. There's not
	// a canonical way of determining if the content is given here as escaped
	// or not, so we use a heuristic.
	contents = maybeUnescapeHtml(contents)

	parsed := maybeParseArticleContent(item.Link)

	if item.Enclosures != nil {
		for _, enc := range item.Enclosures {
			contents = prependMediaToHtml(enc.URL, contents)
			parsed = prependMediaToHtml(enc.URL, parsed)
		}
	}

	if *sanitizeHTML {
		title = bluemondayTitlePolicy.Sanitize(title)
		contents = bluemondayBodyPolicy.Sanitize(contents)
		parsed = bluemondayBodyPolicy.Sanitize(parsed)
	}

	contents = maybeRewriteImageSourceUrls(contents)

	syntheticDate := false
	retrieved := time.Now()
	var date time.Time
	if item.DateValid && !item.Date.IsZero() {
		date = item.Date
	} else {
		log.Warningf("could not find date for item in %s", feed)
		date = retrieved
		syntheticDate = true
	}

	return models.Article{
		FeedID:        feed.ID,
		FolderID:      feed.FolderID,
		Title:         title,
		Summary:       contents,
		Content:       contents,
		Parsed:        parsed,
		Link:          item.Link,
		Date:          date,
		Read:          item.Read,
		Retrieved:     retrieved,
		SyntheticDate: syntheticDate,
	}
}

// maybeResizeImage converts the provided besticon.Icon to a 256x256 PNG image
// and returns an imagePair struct containing the base64-encoded image and
// metadata.
func maybeResizeImage(folderId int64, feedId int64, bi besticon.Icon, i *image.Image) (ip imagePair) {
	ip = imagePair{folderId, feedId, "image/" + bi.Format, bi.ImageData}

	if *normalizeFavicons {
		var buff bytes.Buffer

		resized := image.NewRGBA(image.Rect(0, 0, 256, 256))
		draw.CatmullRom.Scale(resized, resized.Rect, *i, (*i).Bounds(), draw.Over, nil)

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
		log.Warningf("while parsing HTML: %s", err)
		return s
	}

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		for _, attr := range s.Nodes[0].Attr {
			if attr.Key == "src" {

				imgUrl, err := url.Parse(attr.Val)
				if err != nil {
					log.Warningf("while parsing img src %s: %s", attr.Val, err)
				}

				if imgUrl.Scheme == "https" && !*proxySecureImages {
					continue
				}

				newUrl, err := url.Parse(fmt.Sprintf("%s/%s", *proxyUrlBase, cacheEndpoint))
				if err != nil {
					log.Warningf("invalid proxy base URL %s: %s", *proxyUrlBase, err)
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
		log.Warningf("while rendering rewritten HTML: %s", err)
		return s
	}

	return resp
}

// extractTextFromHtml parses the given string as an HTML document and returns
// the combined text from the doc. On parse error, returns the original string.
func extractTextFromHtml(s string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(s))
	if err != nil {
		log.Warningf("while parsing HTML: %s", err)
		return s
	}

	return doc.Text()
}

// prependMediaToHtml prepends the image included in the RSS enclosure to the
// HTML content specified.
func prependMediaToHtml(imgSrc string, s string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(s))
	if err != nil {
		log.Warningf("while parsing HTML: %s", err)
		return s
	}

	imgNode := &html.Node{
		Type:     html.ElementNode,
		Data:     "img",
		DataAtom: atom.Img,
		Attr: []html.Attribute{
			{Key: "src", Val: html.EscapeString(imgSrc)},
		},
	}

	doc.Find("body").First().PrependNodes(imgNode)

	resp, err := doc.Html()
	if err != nil {
		log.Warningf("while rendering rewritten HTML: %s", err)
		return s
	}

	return resp
}

// stemWord returns the stemmed version of the word in English using the Porter
// Stemming algorithm.
func stemWord(s string) string {
	ret := s

	// Filter out some punctuation and other marks. Do not include single quote
	// since stemming should usually handle that.
	reg, err := regexp.Compile(`[.,!?():;"]`)
	if err != nil {
		return ret
	}
	ret = reg.ReplaceAllString(ret, "")

	stemmed, _ := snowball.Stem(ret, "english", true)
	return stemmed
}

// maybeMuteArticle returns true if any of the article's title or contents
// match any of the muted words.
func maybeMuteArticle(a models.Article, muteWords []string, unmuteFeeds []int64) bool {
	muteWordMap := make(map[string]string)

	// If the feed of the article is an unmuted feed, never mute it.
	for _, feedId := range unmuteFeeds {
		if feedId == a.FeedID {
			log.Infof("Not filtering article due to unmuted feed %d: %s", feedId, a.String())
			return false
		}
	}

	for _, word := range muteWords {
		muteWordMap[stemWord(word)] = word
	}

	textWords := strings.Fields(extractTextFromHtml(a.Title))
	textWords = append(textWords, strings.Fields(extractTextFromHtml(a.Summary))...)
	textWords = append(textWords, strings.Fields(extractTextFromHtml(a.Content))...)

	for _, textWord := range textWords {
		if muteWord, ok := muteWordMap[stemWord(textWord)]; ok {
			log.Infof(
				"Filtering article due to muted word \"%s\" -> \"%s\": %s",
				muteWord, textWord, a.String())
			return true
		}
	}

	return false
}

func getSimilarExistingArticles(articles []models.Article, a models.Article) ([]int64, []int64) {
	var unreadIds, readIds []int64

	editDistPercent := func(base string, comp string) float64 {
		edit := float64(levenshtein.Distance(base, comp)) / float64(len(base))
		return edit
	}

	for _, old := range articles {
		if old.Link == a.Link {
			if *strictDedup ||
					(editDistPercent(old.Title, a.Title) < *maxEditDedup &&
							editDistPercent(old.Summary, a.Summary) < *maxEditDedup) {
				if old.Read {
					readIds = append(readIds, old.ID)
				} else {
					unreadIds = append(unreadIds, old.ID)
				}
			}
		}
	}

	return unreadIds, readIds
}
