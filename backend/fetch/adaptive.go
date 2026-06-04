package fetch

import (
	"math"
	"sort"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/rss"
)

// emaAlpha is the smoothing factor for the Exponential Moving Average (EMA)
// of publication gaps. A value of 0.25 places moderate weight on the most
// recent publication interval (25%) while reserving the remaining weight
// (75%) for historical average intervals. This responds relatively quickly
// to diurnal publication changes without causing severe scheduler jitter.
const emaAlpha = 0.25

// calculateAdaptiveInterval computes the optimal duration to wait before the
// next feed fetch based on the history of published articles.
//
// Math:
// 1. Sort article dates descending to get publication timestamps t_0, t_1, ..., t_k.
// 2. Compute publication gaps between adjacent articles: g_i = t_{i-1} - t_i.
// 3. Compute EMA of these gaps:
//    S_k = g_k
//    S_i = alpha * g_i + (1 - alpha) * S_{i+1} for i = k-1 down to 1.
//    Estimated interval T_ema = S_1.
// 4. Calculate silence duration since the newest article: T_silent = now - t_0.
// 5. Raw interval T_raw = max(T_ema, T_silent).
// 6. Return T_raw clamped to minFetchInterval and maxFetchInterval flags.
func calculateAdaptiveInterval(fetchTime time.Time, articles []models.Article) time.Duration {
	var validDates []time.Time
	for _, a := range articles {
		// Filter out invalid dates (zero value) and articles published in the future
		// (which can occur due to misconfigured feed generators or server clock drift).
		if !a.Date.IsZero() && !a.Date.After(fetchTime) {
			validDates = append(validDates, a.Date)
		}
	}

	// Sort dates descending (newest first).
	sort.Slice(validDates, func(i, j int) bool {
		return validDates[i].After(validDates[j])
	})

	// De-duplicate dates. If multiple articles are published at the same second
	// (common during batch uploads or migrations), their gap is zero. Keeping
	// duplicates would artificially deflate the average publication gap.
	var uniqueDates []time.Time
	for _, d := range validDates {
		if len(uniqueDates) == 0 || !d.Equal(uniqueDates[len(uniqueDates)-1]) {
			uniqueDates = append(uniqueDates, d)
		}
	}

	// We need at least 2 distinct dates to calculate at least one gap.
	if len(uniqueDates) < 2 {
		return *minFetchInterval
	}

	// Calculate gaps between consecutive publications. Gaps are positive durations.
	var gaps []time.Duration
	for i := 1; i < len(uniqueDates); i++ {
		gap := uniqueDates[i-1].Sub(uniqueDates[i])
		if gap > 0 {
			gaps = append(gaps, gap)
		}
	}

	if len(gaps) == 0 {
		return *minFetchInterval
	}

	// Compute EMA of gaps starting from the oldest gap (end of the array)
	// to the newest gap (beginning of the array).
	ema := float64(gaps[len(gaps)-1])
	for i := len(gaps) - 2; i >= 0; i-- {
		ema = emaAlpha*float64(gaps[i]) + (1.0-emaAlpha)*ema
	}
	emaDuration := time.Duration(ema)

	// Measure inactivity duration. If the feed has been inactive for a while,
	// back off fetching frequency proportionally.
	timeSinceLatest := fetchTime.Sub(uniqueDates[0])
	if timeSinceLatest < 0 {
		timeSinceLatest = 0
	}

	// The proposed interval is the maximum of the average publication gap and
	// the inactivity duration. This automatically decreases fetch frequency for
	// dead feeds while keeping active ones fresh.
	interval := emaDuration
	if timeSinceLatest > interval {
		interval = timeSinceLatest
	}

	// Clamp within configured limits
	if interval < *minFetchInterval {
		interval = *minFetchInterval
	}
	if interval > *maxFetchInterval {
		interval = *maxFetchInterval
	}

	return interval
}

// calculateNextInterval determines the absolute timestamp for the next fetch.
// It bypasses the adaptive logic and respects the feed's TTL directly if the
// feed explicitly specified a non-default (non-10m) refresh interval.
func (f Fetcher) calculateNextInterval(user models.User, feed models.Feed, fetch *rss.Feed, fetchTime time.Time) time.Time {
	d := fetch.Refresh.Sub(fetchTime)
	// Check if feed provided a custom interval (i.e. not the default 10 minutes,
	// allowing for a 5-second execution latency buffer).
	if math.Abs(d.Seconds()-600.0) > 5.0 {
		log.V(2).Infof("Feed %s %s specifies non-default refresh interval: %s. Respecting it.", user, feed, d)
		return fetch.Refresh
	}

	// Retrieve historical articles for this feed from the database.
	articles, err := f.d.GetArticlesForFeedForUser(user, feed.ID)
	if err != nil {
		log.Warningf("Failed to retrieve article history for feed %d: %s. Using default min interval.", feed.ID, err)
		return fetchTime.Add(*minFetchInterval)
	}

	interval := calculateAdaptiveInterval(fetchTime, articles)
	log.V(2).Infof("Calculated adaptive fetch interval for %s %s: %s", user, feed, interval)
	return fetchTime.Add(interval)
}

// calculateFailureBackoff computes the wait duration after fetch errors.
// It uses exponential backoff: T_min * 2^(consecutiveFailures - 1).
// Capped at 15 failures to prevent time.Duration int64 overflow, and clamped
// to the maxFetchInterval limit.
func (f Fetcher) calculateFailureBackoff(consecutiveFailures int) time.Duration {
	if consecutiveFailures <= 0 {
		return *minFetchInterval
	}

	failures := consecutiveFailures
	// 2^14 * 10m is approx 113 days, which safely fits inside int64 duration
	// and will be correctly clamped to the max interval flag.
	if failures > 15 {
		failures = 15
	}

	multiplier := int64(1) << uint(failures-1)
	backoff := time.Duration(multiplier) * (*minFetchInterval)
	if backoff > *maxFetchInterval {
		backoff = *maxFetchInterval
	}
	return backoff
}
