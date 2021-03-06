package fetch

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/arbovm/levenshtein"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/cache"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/jrupac/rss"
	"github.com/mat/besticon/besticon"
	"github.com/microcosm-cc/bluemonday"
	"io/ioutil"
	"net/url"
	"sync"
	"time"
)

var (
	sanitizeHTML      = flag.Bool("sanitizeHTML", false, "If true, sanitize HTML content with Bluemonday.")
	normalizeFavicons = flag.Bool("normalizeFavicons", true, "If true, resize favicons to 256x256 and encode as PNG.")
	maxEditDedup      = flag.Float64("maxEditDedup", 0.1,
		"The maximum edit distance between articles to be de-duplicated, expressed as percent of edit distance and total length of content.")
)

var (
	pauseChan             = make(chan struct{})
	pauseChanDone         = make(chan struct{})
	resumeChan            = make(chan struct{})
	bluemondayTitlePolicy = bluemonday.StrictPolicy()
	bluemondayBodyPolicy  = makeBodyPolicy()
)

type imagePair struct {
	folderId int64
	id       int64
	mime     string
	favicon  []byte
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
func Start(ctx context.Context, d *storage.Database, r *cache.RetrievalCache) {
	log.Infof("Starting continuous feed fetching.")

	// Add additional time layouts that sometimes appear in feeds.
	rss.TimeLayouts = append(rss.TimeLayouts, "2006-01-02")
	rss.TimeLayouts = append(rss.TimeLayouts, "Monday, 02 Jan 2006 15:04:05 MST")
	rss.TimeLayouts = append(rss.TimeLayouts, "Mon, 02 Jan 2006")

	// Turn off logging of HTTP icon requests.
	besticon.SetLogOutput(ioutil.Discard)

	fctx, cancel := context.WithCancel(ctx)

	// A WaitGroup of size 0 or 1 to wait on all fetching to complete.
	loopCondition := &sync.WaitGroup{}
	loopCondition.Add(1)
	go startFetchLoop(fctx, loopCondition, d, r)

	for {
		select {
		case <-pauseChan:
			cancel()
			loopCondition.Wait()
			log.Info("Fetcher paused.")
			pauseChanDone <- struct{}{}
		case <-resumeChan:
			fctx, cancel = context.WithCancel(ctx)
			loopCondition.Add(1)
			go startFetchLoop(fctx, loopCondition, d, r)
			log.Info("Fetcher resumed.")
		case <-ctx.Done():
			cancel()
			loopCondition.Wait()
			return
		}
	}
}

func startFetchLoop(ctx context.Context, parent *sync.WaitGroup, d *storage.Database, r *cache.RetrievalCache) {
	defer parent.Done()

	users, err := d.GetAllUsers()
	if err != nil {
		log.Fatalf("Cannot start fetcher because fetching users failed: %s", err)
	}

	// A WaitGroup used to wait for all users' fetch loops to complete.
	usersLoopWg := &sync.WaitGroup{}
	usersLoopWg.Add(len(users))

	for _, u := range users {
		go func(u models.User) {
			defer usersLoopWg.Done()
			startFetchLoopForUser(d, ctx, r, u)
		}(u)
	}

	usersLoopWg.Wait()
}

func startFetchLoopForUser(d *storage.Database, ctx context.Context, r *cache.RetrievalCache, u models.User) {
	feeds, err := d.GetAllFeedsForUser(u)
	if err != nil {
		log.Errorf("Failed to fetch all feeds for user %s: %s", u, err)
	}
	utils.DebugPrint(fmt.Sprintf("Feed list for user %s", u), feeds)

	// A WaitGroup of size 0 or 1 to wait on fetching for one user to complete.
	userLoopCondition := &sync.WaitGroup{}
	userLoopCondition.Add(len(feeds))
	ac := make(chan models.Article)
	ic := make(chan imagePair)

	for _, f := range feeds {
		go func(f models.Feed) {
			defer userLoopCondition.Done()
			fetchLoopForFeed(ctx, d, r, ac, ic, f, u)
		}(f)
	}

	for {
		select {
		case a := <-ac:
			utils.DebugPrint("Received a new article:", a)
			if err2 := d.InsertArticleForUser(u, a); err2 != nil {
				log.Warningf("Failed to persist article for user %s: %+v: %s", u, a, err2)
			}
			r.Add(u, a.Hash())
		case ip := <-ic:
			utils.DebugPrint("Received a new image:", ip)
			if err2 := d.InsertFaviconForUser(u, ip.folderId, ip.id, ip.mime, ip.favicon); err2 != nil {
				log.Warningf("Failed to persist icon for user %s for feed %d: %s", u, ip.id, err2)
			}
		case <-ctx.Done():
			log.Infof("Stopping fetching feeds for user %s...", u)
			userLoopCondition.Wait()
			log.Infof("Stopped fetching feeds for user %s.", u)
			return
		}
	}
}

func fetchLoopForFeed(ctx context.Context, d *storage.Database, r *cache.RetrievalCache, ac chan models.Article, ic chan imagePair, feed models.Feed, u models.User) {
	log.Infof("Fetching URL '%s'", feed.URL)
	tick := make(<-chan time.Time)
	initalFetch := make(chan struct{})

	go func() {
		f, err := rss.Fetch(feed.URL)
		if err != nil {
			log.Warningf("Error for user %s feed %d fetching URL '%s': %s", u, feed.ID, feed.URL, err)
			return
		}

		updateFeedMetadataForUser(d, &feed, f, u)

		handleItemsForUser(ctx, &feed, d, r, f.Items, ac, u)
		handleImage(ctx, feed, f, ic)

		tick = time.After(time.Until(f.Refresh))
		log.Infof("Initial waiting to fetch %s until %s for user %s\n", feed.URL, f.Refresh, u)
		initalFetch <- struct{}{}
	}()

	for {
		select {
		case <-initalFetch:
			// Block on initial fetch here so that we can return early if needed
			continue
		case <-tick:
			log.Infof("Fetching feed %s for user %s", feed.URL, u)
			var refresh time.Time
			if f, err := rss.Fetch(feed.URL); err != nil {
				log.Warningf("Error fetching %s: %s", feed.URL, err)
				// If the request transiently fails, try again after a fixed interval.
				refresh = time.Now().Add(10 * time.Minute)
			} else {
				handleItemsForUser(ctx, &feed, d, r, f.Items, ac, u)
				refresh = f.Refresh
			}
			log.Infof("Waiting to fetch %s until %s\n", feed.URL, refresh)
			tick = time.After(time.Until(refresh))
		case <-ctx.Done():
			return
		}
	}
}

func updateFeedMetadataForUser(d *storage.Database, f *models.Feed, rf *rss.Feed, u models.User) {
	if rf.Title != "" {
		f.Title = rf.Title
	}
	if rf.Description != "" {
		f.Description = rf.Description
	}
	if rf.Link != "" {
		f.Link = rf.Link
	}

	err := d.UpdateFeedMetadataForUser(u, *f)
	if err != nil {
		log.Warningf("Error while updated feed metadata for feed %s: %s", f.ID, err)
	}
}

func handleItemsForUser(ctx context.Context, feed *models.Feed, d *storage.Database, r *cache.RetrievalCache, items []*rss.Item, send chan models.Article, u models.User) {
	latest := feed.Latest
	newLatest := latest

	var existingArticles []models.Article
	existingArticles, err := d.GetUnreadArticlesForFeedForUser(u, feed.ID)
	if err != nil {
		log.Warningf("Error while fetching existing articles for feed %s: %s", feed.ID, err)
	}

Loop:
	for _, item := range items {
		title := item.Title
		// Some feeds give back content that is HTML-escaped. When this happens,
		// sanitization makes the content appear as raw, escaped text. There's not
		// a canonical way of determining if the content is given here as escaped
		// or not, so we use a heuristic.
		content := maybeUnescapeHtml(item.Content)
		summary := maybeUnescapeHtml(item.Summary)

		parsed := maybeParseArticleContent(item.Link)

		if *sanitizeHTML {
			title = bluemondayTitlePolicy.Sanitize(title)
			content = bluemondayBodyPolicy.Sanitize(content)
			summary = bluemondayBodyPolicy.Sanitize(summary)
			parsed = bluemondayBodyPolicy.Sanitize(parsed)
		}

		content = maybeRewriteImageSourceUrls(content)
		summary = maybeRewriteImageSourceUrls(summary)

		syntheticDate := false
		retrieved := time.Now()
		var date time.Time
		if item.DateValid && !item.Date.IsZero() {
			date = item.Date
		} else {
			log.V(2).Infof("Could not find date for item: %+v", item)
			date = retrieved
			syntheticDate = true
		}

		a := models.Article{
			FeedID:        feed.ID,
			FolderID:      feed.FolderID,
			Title:         title,
			Summary:       summary,
			Content:       content,
			Parsed:        parsed,
			Link:          item.Link,
			Date:          date,
			Read:          item.Read,
			Retrieved:     retrieved,
			SyntheticDate: syntheticDate,
		}

		if !a.Date.After(latest) {
			log.V(2).Infof("Not persisting too old article: %+v", a)
		} else if r.Lookup(u, a.Hash()) {
			log.V(2).Infof("Not persisting because present in retrieval cache: %+v", a)
		} else {
			// Remove existing articles that are similar to the newly fetched one.
			ids := getSimilarExistingArticles(existingArticles, a)
			if len(ids) > 0 {
				log.Infof("Found %d similar articles to %s: %+v", len(ids), a.Title, ids)
				err = d.DeleteArticlesByIdForUser(u, ids)
				if err != nil {
					log.Warningf("Failed to delete similar articles for feed %s: %s", feed.ID, err)
				}
			}

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
		}
	}

	err = d.UpdateLatestTimeForFeedForUser(u, feed.FolderID, feed.ID, newLatest)
	if err != nil {
		log.Warningf("Failed to update latest feed time: %s", err)
	} else {
		feed.Latest = newLatest
	}
}

func handleImage(ctx context.Context, feed models.Feed, f *rss.Feed, send chan imagePair) {
	var icon besticon.Icon
	var feedHost string

	parsedUrl, err := url.Parse(f.Link)
	if err == nil {
		feedHost = parsedUrl.Hostname()
	}

	if i, err := tryIconFetch(f.Image.URL); err == nil {
		icon = i
	} else if i, err = tryIconFetch(f.Link); err == nil {
		icon = i
	} else if i, err = tryIconFetch(feedHost); err == nil {
		icon = i
	} else {
		log.V(2).Infof("Could not find suitable icon for feed: %s", feedHost)
		return
	}

	select {
	case send <- maybeResizeImage(feed.FolderID, feed.ID, icon):
		break
	case <-ctx.Done():
		break
	}
}

func getSimilarExistingArticles(articles []models.Article, a models.Article) []int64 {
	var ids []int64

	editDistPercent := func(base string, comp string) float64 {
		edit := float64(levenshtein.Distance(base, comp)) / float64(len(base))
		return edit
	}

	for _, old := range articles {
		if old.Link == a.Link {
			if editDistPercent(old.Title, a.Title) < *maxEditDedup &&
				editDistPercent(old.Summary, a.Summary) < *maxEditDedup {
				ids = append(ids, old.ID)
			}
		}
	}

	return ids
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
