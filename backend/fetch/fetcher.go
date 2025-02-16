package fetch

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sync"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/cache"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/jrupac/rss"
	"github.com/mat/besticon/v3/besticon"
	"github.com/microcosm-cc/bluemonday"
)

var (
	sanitizeHTML      = flag.Bool("sanitizeHTML", false, "If true, sanitize HTML content with Bluemonday.")
	normalizeFavicons = flag.Bool("normalizeFavicons", true, "If true, resize favicons to 256x256 and encode as PNG.")
	strictDedup       = flag.Bool("strictDedup", true, "If true, only the link name is used to de-duplicate unread articles.")
	maxEditDedup      = flag.Float64("maxEditDedup", 0.1,
		"The max edit distance between articles to be de-duplicated, expressed as percent of content. If `strictDedup` is set, this is ignored.")
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

type Fetcher struct {
	d        storage.Database
	retCache *cache.RetrievalCache
	finder   *besticon.IconFinder
}

func New(d storage.Database, retCache *cache.RetrievalCache) *Fetcher {
	// Turn off logging of HTTP icon requests.
	b := besticon.New(besticon.WithLogger(besticon.NewDefaultLogger(io.Discard)))

	return &Fetcher{
		d:        d,
		retCache: retCache,
		finder:   b.NewIconFinder(),
	}
}

// Start starts continuous feed fetching and writes fetched articles to the
// database.
func (f Fetcher) Start(ctx context.Context) {
	log.Infof("Starting continuous feed fetching.")

	// Add additional time layouts that sometimes appear in feeds.
	rss.TimeLayouts = append(rss.TimeLayouts, "2006-01-02")
	rss.TimeLayouts = append(rss.TimeLayouts, "Monday, 02 Jan 2006 15:04:05 MST")
	rss.TimeLayouts = append(rss.TimeLayouts, "Mon, 02 Jan 2006 15:04:05 MST")
	rss.TimeLayouts = append(rss.TimeLayouts, "Mon, 2 Jan 2006 15:04:05 MST")
	rss.TimeLayouts = append(rss.TimeLayouts, "Mon, 02 Jan 2006")

	// Create a new cancel-able context since fetching may be paused and resumed
	// separately from the parent context.
	ctx, cancel := context.WithCancel(ctx)

	// A WaitGroup of size 1 to wait on all fetching to complete.
	fetchCond := &sync.WaitGroup{}
	fetchCond.Add(1)
	go f.start(ctx, fetchCond)

	for {
		select {
		case <-pauseChan:
			cancel()
			fetchCond.Wait()
			log.Info("Fetcher paused.")
			pauseChanDone <- struct{}{}
		case <-resumeChan:
			// Create a new context when resuming
			ctx, cancel = context.WithCancel(ctx)
			fetchCond.Add(1)
			go f.start(ctx, fetchCond)
			log.Info("Fetcher resumed.")
		case <-ctx.Done():
			// Explicitly cancel the child context when returning to avoid leaking it
			cancel()
			fetchCond.Wait()
			return
		}
	}
}

func (f Fetcher) start(ctx context.Context, parent *sync.WaitGroup) {
	defer parent.Done()

	users, err := f.d.GetAllUsers()
	if err != nil {
		log.Fatalf("cannot start fetcher because fetching users failed: %s", err)
	}

	// A WaitGroup used to wait for all users' fetch loops to complete.
	userCond := &sync.WaitGroup{}
	userCond.Add(len(users))

	for _, user := range users {
		go f.startFetchForUser(ctx, userCond, user)
	}

	userCond.Wait()
	log.Infof("Stopped feed fetching.")
}

func (f Fetcher) startFetchForUser(ctx context.Context, parent *sync.WaitGroup, user models.User) {
	defer parent.Done()

	feeds, err := f.d.GetAllFeedsForUser(user)
	if err != nil {
		log.Errorf("while fetching all feeds for %s: %s", user, err)
		return
	}

	utils.DebugPrint(fmt.Sprintf("Feed list for %s", user), feeds)

	// A WaitGroup used to wait for all feeds for this user to complete.
	feedCond := &sync.WaitGroup{}
	feedCond.Add(len(feeds))

	for _, feed := range feeds {
		go f.fetchUserFeed(ctx, feedCond, user, feed)
	}

	feedCond.Wait()
	log.Infof("Stopped feed fetching for user %s.", user)
}

func (f Fetcher) fetchUserFeed(ctx context.Context, parent *sync.WaitGroup, user models.User, feed models.Feed) {
	defer parent.Done()

	log.Infof("Starting fetch for:\n\t%s %s", user, feed)
	tick := make(<-chan time.Time)
	firstFetch := make(chan struct{})

	go func() {
		fetch, err := rss.Fetch(feed.URL)
		if err != nil {
			log.Warningf("during first fetch for %s %s: %s", user, feed, err)
			return
		}

		// On only the initial fetch, update feed metadata and favicon
		f.updateFeedMetadataForUser(ctx, user, &feed, fetch)
		f.updateFeedFaviconForUser(ctx, user, &feed, fetch)

		f.processUserFeedItems(ctx, user, &feed, fetch.Items)

		tick = time.After(time.Until(fetch.Refresh))
		log.Infof("First fetch done for %s %s. Waiting until %s", user, feed, fetch.Refresh)

		// Mark the first fetch as complete
		firstFetch <- struct{}{}
	}()

	for {
		select {
		case <-firstFetch:
			// Block on first fetch here so that we can return early if needed.
			// If the first fetch fails, we will wait here indefinitely until the
			// parent context is canceled.
			continue
		case <-tick:
			log.Infof("Fetching %s %s", user, feed)
			var refresh time.Time
			if fetch, err := rss.Fetch(feed.URL); err != nil {
				log.Warningf("while fetching %s %s: %s", user, feed, err)
				// This URL was previously successfully fetched, so this might be a
				// transient failure. Retry at a fixed interval.
				refresh = time.Now().Add(10 * time.Minute)
			} else {
				f.processUserFeedItems(ctx, user, &feed, fetch.Items)
				refresh = fetch.Refresh
			}
			log.Infof("Waiting to fetch %s %s until %s", user, feed, refresh)
			tick = time.After(time.Until(refresh))
		case <-ctx.Done():
			return
		}
	}
}

func (f Fetcher) processUserFeedItems(ctx context.Context, user models.User, feed *models.Feed, items []*rss.Item) {
	prevLatest := feed.Latest
	numTotal := len(items)
	var numInserted, numMarkedRead, numUpdatedExisting, numExistingRemoved, numTooOld, numRetrievalCache, numMuted int

	existingArticles, err := f.d.GetArticlesForFeedForUser(user, feed.ID)
	if err != nil {
		log.Warningf("while fetching existing articles for %s: %s", feed, err)
	}

	muteWords, err := f.d.GetMuteWordsForUser(user)
	if err != nil {
		log.Warningf("while fetching muted words: %s", err)
	}

	for _, item := range items {
		// The context is canceled, so just return
		if ctx.Err() != nil {
			return
		}

		a := processItem(feed, item)

		if !a.Date.After(prevLatest) {
			log.V(2).Infof("Not persisting too old article: %s", a)
			numTooOld += 1
		} else if f.retCache.Lookup(user, a.Hash()) {
			log.V(2).Infof("Not persisting because present in retrieval cache: %s", a)
			numRetrievalCache += 1
		} else if maybeMuteArticle(a, muteWords) {
			log.V(2).Infof("Not persisting because of muted word: %s", a)
			numMuted += 1
		} else {
			numInserted += 1

			// Remove existing articles that are similar to the newly fetched one.
			unreadIds, readIds := getSimilarExistingArticles(existingArticles, a)

			if len(unreadIds) == 0 && len(readIds) > 0 {
				// If all similar articles are read, mark the new one as read too to avoid "resurrecting" it.
				// This is preferable to just skipping it as it progresses the "latest" timestamp.
				log.V(2).Infof("Marking new article for %s read since all similar ones are read: %s", feed, a.Title)
				numMarkedRead += 1
				a.Read = true
			} else if len(unreadIds) > 0 {
				log.V(2).Infof("Found %d similar articles to \"%s\": %+v", len(unreadIds), a.Title, unreadIds)
				numUpdatedExisting += 1
				numExistingRemoved += len(unreadIds)
				err = f.d.DeleteArticlesByIdForUser(user, unreadIds)
				if err != nil {
					log.Warningf("while deleting similar articles for %s: %s", feed, err)
				}
			}

			log.V(2).Infof("Processed for %s a new article: %s", user, a)
			if err = f.d.InsertArticleForUser(user, a); err != nil {
				log.Warningf("while persisting article for %s due to %s: %s", user, err, a)
			} else {
				f.retCache.Add(user, a.Hash())
			}

			if a.Date.After(feed.Latest) {
				err = f.d.UpdateLatestTimeForFeedForUser(user, feed.FolderID, feed.ID, a.Date)
				if err != nil {
					log.Warningf("while updating latest feed time for %s: %s", feed, err)
				} else {
					feed.Latest = a.Date
				}
			}
		}
	}

	log.Infof(
		"Fetch stats:\n\t%s %s\n\ttotal=%d, inserted=%d (marked read=%d, updated existing=%d, existing removed=%d), too old=%d, retrieval cache=%d, muted=%d",
		user, feed, numTotal, numInserted, numMarkedRead, numUpdatedExisting, numExistingRemoved, numTooOld, numRetrievalCache, numMuted)
}
