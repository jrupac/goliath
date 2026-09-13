package fetch

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/cache"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/jrupac/rss"
	"github.com/mat/besticon/v3/besticon"
	"github.com/microcosm-cc/bluemonday"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	sanitizeHTML      = flag.Bool("sanitizeHTML", false, "If true, sanitize HTML content with Bluemonday.")
	normalizeFavicons = flag.Bool("normalizeFavicons", true, "If true, resize favicons to 256x256 and encode as PNG.")
	strictDedup       = flag.Bool("strictDedup", true, "If true, only the link name is used to de-duplicate unread articles.")
	maxEditDedup      = flag.Float64("maxEditDedup", 0.1,
		"The max edit distance between articles to be de-duplicated, expressed as percent of content. If `strictDedup` is set, this is ignored.")
	feedAllowedAddresses = flag.String("feedAllowedAddresses", "",
		"Comma-separated host:port addresses off the public internet that feed fetching may reach anyway, "+
			"for a feed bridge run alongside this server.")

	userAgentFlag = flag.String("userAgent", "Goliath/1.0 (+http://github.com/jrupac/goliath)",
		"User-Agent header sent on every request this server makes to a site it does not own.")

	minFetchInterval = flag.Duration("minFetchInterval", 10*time.Minute, "Minimum interval between feed fetches.")
	maxFetchInterval = flag.Duration("maxFetchInterval", 24*time.Hour, "Maximum interval between feed fetches.")
	emaAlphaFaster   = flag.Float64("emaAlphaFaster", 0.5,
		"EMA smoothing factor applied when the observed publication gap is shorter than the current estimated interval (feed is becoming more active). Higher values converge faster toward a shorter cadence.")
	emaAlphaSlower = flag.Float64("emaAlphaSlower", 0.1,
		"EMA smoothing factor applied when the observed publication gap is longer than the current estimated interval (feed appears to be slowing down). Lower values resist upward drift from anomalous gaps such as silence periods or server restarts.")
	maxGapEMAMultiple = flag.Float64("maxGapEMAMultiple", 3.0,
		"Maximum ratio by which a single observed publication gap may exceed the current estimated interval before being clamped for the EMA update. Prevents a single long quiet period from disproportionately inflating the estimate in one step.")
	feedMetadataRefreshInterval = flag.Duration("feedMetadataRefreshInterval", 6*time.Hour,
		"Interval between refreshes of a feed's own title, description, site link and favicon.")
)

var (
	bluemondayTitlePolicy = bluemonday.StrictPolicy()
	bluemondayBodyPolicy  = makeBodyPolicy()
)

var (
	feedFetchIntervalMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "feed_fetch_interval_seconds",
			Help: "Calculated adaptive fetch interval in seconds on a per-user, per-feed basis.",
		},
		[]string{"username", "feed_id", "feed_title", "feed_url"},
	)
	feedFetchConsecutiveFailuresMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "feed_fetch_consecutive_failures",
			Help: "Current number of consecutive fetch failures on a per-user, per-feed basis.",
		},
		[]string{"username", "feed_id", "feed_title", "feed_url"},
	)
	feedFetchErrorsMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feed_fetch_errors_total",
			Help: "Total number of fetch errors on a per-user, per-feed basis.",
		},
		[]string{"username", "feed_id", "feed_title", "feed_url"},
	)
	feedFetchAttemptsMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feed_fetch_attempts_total",
			Help: "Total number of fetch attempts on a per-user, per-feed basis.",
		},
		[]string{"username", "feed_id", "feed_title", "feed_url"},
	)
	feedFetchStatsMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feed_fetch_stats_total",
			Help: "Per-fetch item disposition counts on a per-user, per-feed basis. The 'stat' label identifies the disposition: total (items seen in feed response), inserted (new articles persisted), marked_read_auto (inserted but auto-marked read due to dedup), updated_existing (replaced a similar existing article), existing_removed (existing articles deleted by dedup), too_old (older than last known article), retrieval_cache_hit (already seen via cache), muted (suppressed by mute rules).",
		},
		[]string{"username", "feed_id", "feed_title", "feed_url", "stat"},
	)
)

var feedFetchStatKeys = []string{
	"total", "inserted", "marked_read_auto", "updated_existing",
	"existing_removed", "too_old", "retrieval_cache_hit", "muted",
}

func init() {
	prometheus.MustRegister(feedFetchIntervalMetric)
	prometheus.MustRegister(feedFetchConsecutiveFailuresMetric)
	prometheus.MustRegister(feedFetchErrorsMetric)
	prometheus.MustRegister(feedFetchAttemptsMetric)
	prometheus.MustRegister(feedFetchStatsMetric)

	// Additional time layouts that sometimes appear in feeds.
	rss.TimeLayouts = append(rss.TimeLayouts,
		"2006-01-02",
		"Monday, 02 Jan 2006 15:04:05 MST",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"Mon, 02 Jan 2006",
	)
}

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

// SanitizeBody sanitizes html body content using the default bluemonday policy.
func SanitizeBody(html string) string {
	return bluemondayBodyPolicy.Sanitize(html)
}

// feedFetchTimeout bounds a single feed fetch.
const feedFetchTimeout = 10 * time.Second

// UserAgent returns how this server identifies itself when fetching from a
// site it does not own.
//
// One value for every outbound request, so that a publisher deciding whether
// to serve, block or rate limit Goliath is deciding about one client rather
// than about several that happen to be the same program.
func UserAgent() string {
	return *userAgentFlag
}

// NewFeedAllowlist builds the set of otherwise-refused addresses feed fetching
// may reach, from the configured flag.
//
// Separate from the fetcher so that a malformed allowlist is reported where
// the rest of the configuration is checked, rather than by a fetch failing
// later for a reason that looks like the feed's fault.
func NewFeedAllowlist() (utils.AddressAllowlist, error) {
	return utils.NewAddressAllowlist(strings.Split(*feedAllowedAddresses, ","))
}

// newFeedClient returns the client feeds are fetched with.
//
// Guarded like any other fetch of a URL this process did not choose: a
// subscription's URL comes from whoever added it, and unlike a one-off request
// it is fetched again on every cycle. A feed bridge run alongside this server
// is the legitimate case for reaching a private address, and it is configured
// rather than inferred.
func newFeedClient(allowed utils.AddressAllowlist) *http.Client {
	return &http.Client{
		Timeout:   feedFetchTimeout,
		Transport: utils.GuardedTransport("Feed fetch", feedFetchTimeout, allowed),
	}
}

func fetchFuncWithClient(client *http.Client) rss.FetchFunc {
	return func(url string) (*http.Response, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/rss+xml,application/atom+xml;q=0.9,application/xml;q=0.8,*/*;q=0.7")
		req.Header.Set("User-Agent", UserAgent())
		return client.Do(req)
	}
}

type Fetcher struct {
	d         storage.Database
	retCache  cache.RetrievalCache
	finder    IconFinder
	fetchFunc rss.FetchFunc
}

func New(d storage.Database, retCache cache.RetrievalCache, allowed utils.AddressAllowlist) *Fetcher {
	// Turn off logging of HTTP icon requests.
	b := besticon.New(besticon.WithLogger(besticon.NewDefaultLogger(io.Discard)))

	return &Fetcher{
		d:         d,
		retCache:  retCache,
		finder:    b.NewIconFinder(),
		fetchFunc: fetchFuncWithClient(newFeedClient(allowed)),
	}
}

// task is one fetch of one feed for one user, as the scheduler hands it to a
// worker. It carries only what the scheduler remembers between fetches; the
// feed itself is read fresh, so that a move, a rename or an unsubscribe made
// since the last fetch is what this one sees.
type task struct {
	// ctx is cancelled when the feed is unscheduled while the task runs.
	ctx             context.Context
	key             storage.UserFeedKey
	user            models.User
	failures        int
	refreshMetadata bool
}

// outcome is what a worker reports about a task once it is done.
type outcome struct {
	key storage.UserFeedKey
	// next is when the feed is next due.
	next time.Time
	// failures is the count of consecutive failed fetches, this one included.
	failures          int
	refreshedMetadata bool
	// gone reports that the feed is no longer subscribed to, so it is not to
	// be fetched again.
	gone bool
	// labels are the per-feed metric labels this fetch wrote under, so that
	// the series can be deleted once the feed goes or its labels change.
	labels []string
}

// fetchFeed fetches one feed and stores whatever is new in it.
func (f Fetcher) fetchFeed(t task) outcome {
	o := outcome{key: t.key, failures: t.failures}
	user := t.user

	feed, err := f.d.GetFeedForUser(user, t.key.FeedID)
	if errors.Is(err, sql.ErrNoRows) {
		log.Infof("Feed %d is no longer subscribed to by %s; no longer fetching it", t.key.FeedID, user)
		o.gone = true
		return o
	}
	fetchTime := time.Now()
	if err != nil {
		log.Warningf("while reading feed %d for %s: %s", t.key.FeedID, user, err)
		o.failures++
		o.next = fetchTime.Add(f.calculateFailureBackoff(o.failures))
		return o
	}

	o.labels = []string{user.Username, strconv.FormatInt(feed.ID, 10), feed.Title, feed.URL}
	feedFetchAttemptsMetric.WithLabelValues(o.labels...).Inc()
	log.Infof("Fetching %s %s", user, feed)

	fetched, err := rss.FetchByFunc(f.fetchFunc, feed.URL)
	if err != nil {
		log.Warningf("while fetching %s %s: %s", user, feed, err)
		o.failures++
		o.next = fetchTime.Add(f.calculateFailureBackoff(o.failures))
		feedFetchErrorsMetric.WithLabelValues(o.labels...).Inc()
	} else {
		if t.refreshMetadata {
			f.updateFeedMetadataForUser(t.ctx, user, &feed, fetched)
			f.updateFeedFaviconForUser(t.ctx, user, &feed, fetched)
			o.refreshedMetadata = true
		}

		if err = f.processUserFeedItems(t.ctx, user, &feed, fetched.Items); err != nil {
			if errors.Is(err, storage.ErrFeedGone) {
				log.Infof("Feed %s was unsubscribed from by %s during a fetch; no longer fetching it", feed, user)
				o.gone = true
			}
			// Otherwise cancelled, because the feed was unscheduled or fetching
			// is stopping, and nothing will read the rest of the outcome.
			return o
		}
		o.failures = 0
		o.next = f.calculateNextInterval(user, &feed, fetched, fetchTime)
	}

	interval := o.next.Sub(fetchTime)
	feedFetchIntervalMetric.WithLabelValues(o.labels...).Set(interval.Seconds())
	feedFetchConsecutiveFailuresMetric.WithLabelValues(o.labels...).Set(float64(o.failures))
	log.Infof("Waiting to fetch %s %s until %s (interval: %s, consecutive failures: %d)",
		user, feed, o.next, interval, o.failures)
	return o
}

// deleteFeedMetrics removes the per-feed series written under the given
// labels.
func deleteFeedMetrics(labels []string) {
	if labels == nil {
		return
	}
	feedFetchIntervalMetric.DeleteLabelValues(labels...)
	feedFetchConsecutiveFailuresMetric.DeleteLabelValues(labels...)
	feedFetchErrorsMetric.DeleteLabelValues(labels...)
	feedFetchAttemptsMetric.DeleteLabelValues(labels...)
	for _, stat := range feedFetchStatKeys {
		feedFetchStatsMetric.DeleteLabelValues(append(slices.Clone(labels), stat)...)
	}
}

// processUserFeedItems stores the new items of a fetched feed. It stops early,
// returning why, if the context is cancelled or the feed turns out to have
// been unsubscribed from, in which case storage.ErrFeedGone is returned.
func (f Fetcher) processUserFeedItems(ctx context.Context, user models.User, feed *models.Feed, items []*rss.Item) error {
	prevLatest := feed.Latest
	numTotal := len(items)
	var numInserted, numMarkedRead, numUpdatedExisting, numExistingRemoved, numTooOld, numRetrievalCache, numMuted int

	existingArticles, err := f.d.GetArticlesForFeedForUser(user, feed.ID)
	if err != nil {
		log.Warningf("while fetching existing articles for %s: %s", feed, err)
	}

	muteWords, err := f.d.GetMuteWordsForUser(user)
	if err != nil {
		log.Warningf("while fetching muted words for user %s: %s", user, err)
	}

	unmuteFeeds, err := f.d.GetUnmuteFeedsForUser(user)
	if err != nil {
		log.Warningf("while fetching unmuted feeds for user %s: %s", user, err)
	}

	feedRegexStrings, err := f.d.GetMuteRegexesForFeedForUser(user, feed.ID)
	if err != nil {
		log.Warningf("while fetching feed mute regexes for user %s, feed %d: %s", user, feed.ID, err)
	}
	var feedRegexes []*regexp.Regexp
	for _, rStr := range feedRegexStrings {
		// Prepend (?i) to ensure case-insensitive matching
		r, err := regexp.Compile("(?i)" + rStr)
		if err != nil {
			log.Warningf("invalid feed mute regex %q: %s", rStr, err)
			continue
		}
		feedRegexes = append(feedRegexes, r)
	}

	for _, item := range items {
		if err = ctx.Err(); err != nil {
			return err
		}

		a := processItem(feed, item)

		if !a.Date.After(prevLatest) {
			log.V(2).Infof("Not persisting too old article: %s", a)
			numTooOld += 1
		} else if f.retCache.Lookup(user, feed.ID, a.Hash()) {
			log.V(2).Infof("Not persisting because present in retrieval cache: %s", a)
			numRetrievalCache += 1
		} else if maybeMuteArticleByRegex(a, feedRegexes) {
			log.V(2).Infof("Not persisting because of feed mute regex: %s", a)
			numMuted += 1
		} else if maybeMuteArticle(a, muteWords, unmuteFeeds) {
			log.V(2).Infof("Not persisting because of muted word: %s", a)
			numMuted += 1
		} else {
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
			// Counted here rather than where the article was accepted for
			// insertion, so that the reported total is what was stored and not
			// what was attempted. A failed insert reading as a success is how
			// a feed comes to look like it is being filled while staying empty.
			if err = f.d.InsertArticleForUser(user, a); errors.Is(err, storage.ErrFeedGone) {
				return err
			} else if err != nil {
				log.Warningf("while persisting article for %s due to %s: %s", user, err, a)
			} else {
				numInserted += 1
				f.retCache.Add(user, feed.ID, a.Hash())
			}

			if a.Date.After(feed.Latest) {
				err = f.d.UpdateLatestTimeForFeedForUser(user, feed.ID, a.Date)
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

	feedIDStr := strconv.FormatInt(feed.ID, 10)
	statCounts := []int{numTotal, numInserted, numMarkedRead, numUpdatedExisting, numExistingRemoved, numTooOld, numRetrievalCache, numMuted}
	for i, stat := range feedFetchStatKeys {
		if statCounts[i] > 0 {
			feedFetchStatsMetric.WithLabelValues(user.Username, feedIDStr, feed.Title, feed.URL, stat).Add(float64(statCounts[i]))
		}
	}
	return nil
}
