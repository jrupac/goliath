package fetch

import (
	"math"
	"sort"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/rss"
)

// calculateNextInterval determines the absolute timestamp for the next fetch.
// It uses an Online Exponential Moving Average (EMA) of publication gaps,
// persisted in the database, to adaptively schedule updates.
//
// Math & Logic:
// 1. Identify any "new" items in the fetched feed: items with a valid publication
//    timestamp strictly after the feed's previous latest timestamp.
// 2. If new items exist, sort them chronologically (oldest to newest) to process gaps.
// 3. Compute publication gaps:
//    - Gap 0: oldest new item date minus prev feed latest date (if valid).
//    - Gap i: date of new item i minus date of new item i-1.
// 4. Update the estimated_refresh_interval in the DB step-by-step. Each gap is
//    first clamped to maxGapEMAMultiple * currentEMA to prevent a single anomalous
//    observation from inflating the estimate. Then an asymmetric alpha is applied:
//    emaAlphaFaster when the gap is shorter than the current EMA (feed speeding up),
//    emaAlphaSlower when longer (resist upward drift from silence or outlier gaps).
// 5. If no new items are found, the estimated_refresh_interval remains unchanged.
// 6. Schedule the next fetch at now + max(EMA, cappedSilence), clamped to [min, max].
//    cappedSilence = min(timeSinceLatest, maxGapEMAMultiple * EMA). Capping bounds
//    the feedback loop for bursty feeds while still backing off truly silent ones.
//    Feeds with no publication history (Latest is zero) skip the silence penalty.
//    Feeds with no stored EMA default to maxFetchInterval/2 as a conservative start.
func (f Fetcher) calculateNextInterval(user models.User, feed *models.Feed, fetch *rss.Feed, fetchTime time.Time) time.Time {
	// First, check if the feed provided a custom non-default interval (like a TTL).
	d := fetch.Refresh.Sub(fetchTime)
	if math.Abs(d.Seconds()-600.0) > 5.0 {
		log.V(2).Infof("Feed %s %s specifies non-default refresh interval: %s. Respecting it.", user, feed, d)
		return fetch.Refresh
	}

	// 1. Identify new items since the feed's last known latest article
	var newItems []*rss.Item
	for _, item := range fetch.Items {
		if item.DateValid && !item.Date.IsZero() && !item.Date.After(fetchTime) {
			// If feed.Latest is zero, all valid items are considered "new" for bootstrapping
			if feed.Latest.IsZero() || item.Date.After(feed.Latest) {
				newItems = append(newItems, item)
			}
		}
	}

	// 2. Sort new items chronologically (oldest first)
	sort.Slice(newItems, func(i, j int) bool {
		return newItems[i].Date.Before(newItems[j].Date)
	})

	// De-duplicate timestamps to avoid zero-gap anomalies
	var uniqueNewDates []time.Time
	for _, item := range newItems {
		d := item.Date
		if len(uniqueNewDates) == 0 || !d.Equal(uniqueNewDates[len(uniqueNewDates)-1]) {
			uniqueNewDates = append(uniqueNewDates, d)
		}
	}

	// Read current stored EMA. Default to half of maxFetchInterval for feeds
	// with no publication history, as a conservative starting point that
	// converges down once articles are observed.
	emaSeconds := feed.EstimatedRefreshInterval
	if emaSeconds <= 0 {
		emaSeconds = int((*maxFetchInterval / 2).Seconds())
	}

	// applyAlpha clamps gap to maxGapEMAMultiple * currentEMA then applies
	// emaAlphaFaster or emaAlphaSlower depending on whether the gap is shorter
	// or longer than the current EMA.
	applyAlpha := func(gap, currentEMA time.Duration) time.Duration {
		if currentEMA > 0 {
			if cap := time.Duration(float64(currentEMA) * *maxGapEMAMultiple); gap > cap {
				gap = cap
			}
		}
		alpha := *emaAlphaFaster
		if gap > currentEMA {
			alpha = *emaAlphaSlower
		}
		return time.Duration(alpha*float64(gap) + (1.0-alpha)*float64(currentEMA))
	}

	// 3 & 4. Compute updated EMA step-by-step
	if len(uniqueNewDates) > 0 {
		var newEmaSeconds int

		if feed.Latest.IsZero() {
			// Bootstrap: Calculate gaps purely between the new items
			if len(uniqueNewDates) < 2 {
				newEmaSeconds = emaSeconds
			} else {
				ema := uniqueNewDates[1].Sub(uniqueNewDates[0])
				for i := 2; i < len(uniqueNewDates); i++ {
					gap := uniqueNewDates[i].Sub(uniqueNewDates[i-1])
					ema = applyAlpha(gap, ema)
				}
				newEmaSeconds = int(ema.Seconds())
			}
		} else {
			// Standard update: compute first gap against previous Latest, then subsequent gaps
			gap0 := uniqueNewDates[0].Sub(feed.Latest)
			ema := time.Duration(emaSeconds) * time.Second
			ema = applyAlpha(gap0, ema)

			for i := 1; i < len(uniqueNewDates); i++ {
				gap := uniqueNewDates[i].Sub(uniqueNewDates[i-1])
				ema = applyAlpha(gap, ema)
			}
			newEmaSeconds = int(ema.Seconds())
		}

		// Update database and local copy if the estimate changed
		if newEmaSeconds != emaSeconds {
			log.V(2).Infof("Updating estimated refresh interval for %s %s to %s", user, feed, time.Duration(newEmaSeconds)*time.Second)
			err := f.d.UpdateEstimatedRefreshIntervalForFeedForUser(user, feed.FolderID, feed.ID, newEmaSeconds)
			if err != nil {
				log.Warningf("Failed to update estimated refresh interval in DB: %s", err)
			} else {
				feed.EstimatedRefreshInterval = newEmaSeconds
			}
		}
	}

	// 5. Schedule the next fetch using EMA, with a capped silence penalty.
	emaDuration := time.Duration(feed.EstimatedRefreshInterval) * time.Second
	if emaDuration <= 0 {
		emaDuration = time.Duration(emaSeconds) * time.Second
	}

	interval := emaDuration
	// Re-apply a silence penalty (time since last article) as a scheduling
	// floor, capped at maxGapEMAMultiple * EMA. This backs off silent feeds
	// without allowing an unbounded spiral for bursty feeds.
	if !feed.Latest.IsZero() {
		timeSinceLatest := fetchTime.Sub(feed.Latest)
		if timeSinceLatest < 0 {
			timeSinceLatest = 0
		}
		if cap := time.Duration(float64(emaDuration) * *maxGapEMAMultiple); timeSinceLatest > cap {
			timeSinceLatest = cap
		}
		if timeSinceLatest > interval {
			interval = timeSinceLatest
		}
	}
	if interval < *minFetchInterval {
		interval = *minFetchInterval
	}
	if interval > *maxFetchInterval {
		interval = *maxFetchInterval
	}

	return fetchTime.Add(interval)
}

// calculateFailureBackoff computes retry duration on fetch failure.
func (f Fetcher) calculateFailureBackoff(consecutiveFailures int) time.Duration {
	if consecutiveFailures <= 0 {
		return *minFetchInterval
	}

	failures := consecutiveFailures
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
