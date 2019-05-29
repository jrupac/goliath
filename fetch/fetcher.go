package fetch

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"github.com/SlyMarbo/rss"
	"github.com/disintegration/imaging"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/mat/besticon/besticon"
	"github.com/microcosm-cc/bluemonday"
	"html"
	"image/png"
	"io/ioutil"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	parseArticles     = flag.Bool("parseArticles", false, "If true, parse article content via Mercury API.")
	sanitizeHTML      = flag.Bool("sanitizeHTML", false, "If true, sanitize HTML content with Bluemonday.")
	normalizeFavicons = flag.Bool("normalizeFavicons", true, "If true, resize favicons to 256x256 and encode as PNG.")
)

var (
	pauseChan             = make(chan struct{})
	pauseChanDone         = make(chan struct{})
	resumeChan            = make(chan struct{})
	bluemondayTitlePolicy = bluemonday.StrictPolicy()
	bluemondayBodyPolicy  = makeBodyPolicy()
)

type imagePair struct {
	id      int64
	mime    string
	favicon []byte
}

func makeBodyPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("title", "alt").OnElements("img")
	return p
}

// Pause stops all continuous feed fetching in a way that is resume-able.
// This call will block until fetching is fully paused. If fetching has not
// started yet, this call will block indefinitely.
func Pause() {
	pauseChan <- struct{}{}
	<-pauseChanDone
}

// Resume resumes continuous feed fetching with a fresh read of feeds.
// If fetching has not started yet, this call will block indefinitely.
func Resume() {
	resumeChan <- struct{}{}
}

// Start starts continuous feed fetching and writes fetched articles to the
// database.
func Start(ctx context.Context, d *storage.Database) {
	log.Infof("Starting continuous feed fetching.")

	// Add additional time layouts that sometimes appear in feeds.
	rss.TimeLayouts = append(rss.TimeLayouts, "2006-01-02")
	rss.TimeLayouts = append(rss.TimeLayouts, "Monday, 02 Jan 2006 15:04:05 MST")
	rss.TimeLayouts = append(rss.TimeLayouts, "Mon, 02 Jan 2006")

	// Turn off logging of HTTP icon requests.
	besticon.SetLogOutput(ioutil.Discard)

	fctx, cancel := context.WithCancel(ctx)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go start(fctx, wg, d)

	for {
		select {
		case <-pauseChan:
			cancel()
			wg.Wait()
			log.Info("Fetcher paused.")
			pauseChanDone <- struct{}{}
		case <-resumeChan:
			fctx, cancel = context.WithCancel(ctx)
			wg.Add(1)
			go start(fctx, wg, d)
			log.Info("Fetcher resumed.")
		case <-ctx.Done():
			wg.Wait()
			cancel()
			return
		}
	}
}

func start(ctx context.Context, parent *sync.WaitGroup, d *storage.Database) {
	defer parent.Done()

	feeds, err := d.GetAllFeeds()
	if err != nil {
		log.Errorf("Failed to fetch all feeds: %s", err)
	}
	utils.DebugPrint("Feed list", feeds)

	wg := &sync.WaitGroup{}
	wg.Add(len(feeds))
	ac := make(chan models.Article)
	ic := make(chan imagePair)

	for _, f := range feeds {
		go func(f models.Feed) {
			defer wg.Done()
			fetchLoop(ctx, d, ac, ic, f)
		}(f)
	}

	for {
		select {
		case a := <-ac:
			utils.DebugPrint("Received a new article:", a)
			if err2 := d.InsertArticle(a); err2 != nil {
				log.Warningf("Failed to persist article: %+v: %s", a, err2)
			}
		case ip := <-ic:
			utils.DebugPrint("Received a new image:", ip)
			if err2 := d.InsertFavicon(ip.id, ip.mime, ip.favicon); err2 != nil {
				log.Warningf("Failed to persist icon for feed %d: %s", ip.id, err2)
			}
		case <-ctx.Done():
			log.Infof("Stopping fetching feeds...")
			wg.Wait()
			log.Infof("Stopped fetching feeds.")
			return
		}
	}
}

func fetchLoop(ctx context.Context, d *storage.Database, ac chan models.Article, ic chan imagePair, feed models.Feed) {
	log.Infof("Fetching URL '%s'", feed.URL)
	tick := make(<-chan time.Time)
	initalFetch := make(chan struct{})

	go func() {
		f, err := rss.Fetch(feed.URL)
		if err != nil {
			log.Warningf("Error for feed %d fetching URL '%s': %s", feed.ID, feed.URL, err)
			return
		}
		handleItems(ctx, &feed, d, f.Items, ac)
		handleImage(ctx, feed, f, ic)

		tick = time.After(time.Until(f.Refresh))
		log.Infof("Initial waiting to fetch %s until %s\n", feed.URL, f.Refresh)
		initalFetch <- struct{}{}
	}()

	for {
		select {
		case <-initalFetch:
			// Block on initial fetch here so that we can return early if needed
			continue
		case <-tick:
			log.Infof("Fetching feed %s", feed.URL)
			var refresh time.Time
			if f, err := rss.Fetch(feed.URL); err != nil {
				log.Warningf("Error fetching %s: %s", feed.URL, err)
				// If the request transiently fails, try again after a fixed interval.
				refresh = time.Now().Add(10 * time.Minute)
			} else {
				handleItems(ctx, &feed, d, f.Items, ac)
				refresh = f.Refresh
			}
			log.Infof("Waiting to fetch %s until %s\n", feed.URL, refresh)
			tick = time.After(time.Until(refresh))
		case <-ctx.Done():
			return
		}
	}
}

func handleItems(ctx context.Context, feed *models.Feed, d *storage.Database, items []*rss.Item, send chan models.Article) {
	latest := feed.Latest
	newLatest := latest

Loop:
	for _, item := range items {
		title := item.Title
		// Some feeds give back content that is HTML-escaped. When this happens,
		// sanitization makes the content appear as raw, escaped text. There's not
		// a canonical way of determining if the content is given here as escaped
		// or not, so we use a heuristic.
		content := maybeUnescapeHtml(item.Content)
		summary := maybeUnescapeHtml(item.Summary)

		parsed := ""
		if *parseArticles {
			if p, err := parseArticleContent(item.Link); err != nil {
				log.Warningf("Parsing content failed: %s", err)
			} else {
				parsed = p
			}
		}
		if *sanitizeHTML {
			title = bluemondayTitlePolicy.Sanitize(title)
			content = bluemondayBodyPolicy.Sanitize(content)
			summary = bluemondayBodyPolicy.Sanitize(summary)
			parsed = bluemondayBodyPolicy.Sanitize(parsed)
		}

		a := models.Article{
			FeedID:    feed.ID,
			FolderID:  feed.FolderID,
			Title:     title,
			Summary:   summary,
			Content:   content,
			Parsed:    parsed,
			Link:      item.Link,
			Date:      item.Date,
			Read:      item.Read,
			Retrieved: time.Now(),
		}

		if a.Date.After(latest) {
			select {
			case send <- a:
				break
			case <-ctx.Done():
				// Break out of processing articles and just clean up.
				break Loop
			}
			if a.Date.After(newLatest) {
				newLatest = a.Date
			}
		} else {
			log.V(2).Infof("Not persisting too old article: %+v", a)
		}
	}

	err := d.UpdateLatestTimeForFeed(feed.ID, newLatest)
	if err != nil {
		log.Warningf("Failed to update latest feed time: %s", err)
	} else {
		feed.Latest = newLatest
	}
}

func handleImage(ctx context.Context, feed models.Feed, f *rss.Feed, send chan imagePair) {
	var icon besticon.Icon
	var feedHost string

	u, err := url.Parse(f.Link)
	if err == nil {
		feedHost = u.Hostname()
	}

	if i, err2 := tryIconFetch(f.Image.URL); err2 == nil {
		icon = i
	} else if i, err2 = tryIconFetch(f.Link); err2 == nil {
		icon = i
	} else if i, err2 = tryIconFetch(feedHost); err2 == nil {
		icon = i
	} else {
		return
	}

	select {
	case send <- maybeResizeImage(feed.ID, icon):
		break
	case <-ctx.Done():
		break
	}
}

func tryIconFetch(link string) (besticon.Icon, error) {
	icon := besticon.Icon{}

	if link == "" {
		return icon, errors.New("invalid URL")
	}

	finder := besticon.IconFinder{}

	icons, err := finder.FetchIcons(link)
	if err != nil {
		return icon, err
	}

	if len(icons) == 0 {
		return icon, errors.New("no icons found")
	}

	for _, i := range icons {
		if i.URL != "" && i.Format != "" {
			return i, nil
		}
	}

	return icon, errors.New("no suitable icons found")
}

func maybeResizeImage(feedId int64, bi besticon.Icon) (ip imagePair) {
	ip = imagePair{feedId, "image/" + bi.Format, bi.ImageData}

	if *normalizeFavicons {
		var buff bytes.Buffer

		i, err := bi.Image()
		if err != nil {
			log.Warningf("failed to convert besticon.Icon to image.Image: %s", err)
			return
		}

		resized := imaging.Resize(*i, 256, 256, imaging.Lanczos)

		err = png.Encode(&buff, resized)
		if err != nil {
			log.Warningf("failed to encode image as PNG: %s", err)
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
