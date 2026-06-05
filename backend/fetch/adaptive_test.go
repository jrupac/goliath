package fetch

import (
	"math"
	"testing"
	"time"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/rss"
)

func TestCalculateFailureBackoff(t *testing.T) {
	oldMin := *minFetchInterval
	oldMax := *maxFetchInterval
	*minFetchInterval = 10 * time.Minute
	*maxFetchInterval = 24 * time.Hour
	defer func() {
		*minFetchInterval = oldMin
		*maxFetchInterval = oldMax
	}()

	f := Fetcher{}

	tests := []struct {
		failures int
		expected time.Duration
	}{
		{failures: -1, expected: 10 * time.Minute},
		{failures: 0, expected: 10 * time.Minute},
		{failures: 1, expected: 10 * time.Minute},
		{failures: 2, expected: 20 * time.Minute},
		{failures: 3, expected: 40 * time.Minute},
		{failures: 4, expected: 80 * time.Minute},
		{failures: 8, expected: 1280 * time.Minute}, // ~21.3h
		{failures: 9, expected: 24 * time.Hour},     // 2560m clamped to 24h
		{failures: 100, expected: 24 * time.Hour},   // clamped and safe from overflow
	}

	for _, tc := range tests {
		got := f.calculateFailureBackoff(tc.failures)
		if got != tc.expected {
			t.Errorf("failures %d: expected %s, got %s", tc.failures, tc.expected, got)
		}
	}
}

func TestCalculateNextInterval(t *testing.T) {
	oldMin := *minFetchInterval
	oldMax := *maxFetchInterval
	*minFetchInterval = 10 * time.Minute
	*maxFetchInterval = 24 * time.Hour
	defer func() {
		*minFetchInterval = oldMin
		*maxFetchInterval = oldMax
	}()

	now := time.Now()
	user := models.User{UserId: "test-user"}

	t.Run("respects custom non-10m refresh directly", func(t *testing.T) {
		db := &storage.MockDB{}
		f := Fetcher{d: db}
		feed := models.Feed{ID: 123}

		customRefresh := now.Add(3 * time.Hour)
		rssFeed := &rss.Feed{
			Refresh: customRefresh,
		}

		got := f.calculateNextInterval(user, &feed, rssFeed, now)
		if !got.Equal(customRefresh) {
			t.Errorf("expected custom refresh %s to be respected, got %s", customRefresh, got)
		}
	})

	t.Run("bootstraps new feed with multiple items (Latest is zero)", func(t *testing.T) {
		var updatedInterval int
		db := &storage.MockDB{}
		db.OnUpdateEstimatedRefreshIntervalForFeedForUser = func(u models.User, folderId, id int64, interval int) error {
			updatedInterval = interval
			return nil
		}
		f := Fetcher{d: db}
		feed := models.Feed{ID: 123, Latest: time.Time{}} // Latest is zero

		rssFeed := &rss.Feed{
			Refresh: now.Add(10 * time.Minute),
			Items: []*rss.Item{
				{Date: now.Add(-1 * time.Hour), DateValid: true},
				{Date: now.Add(-3 * time.Hour), DateValid: true},
				{Date: now.Add(-7 * time.Hour), DateValid: true},
				{Date: now.Add(-15 * time.Hour), DateValid: true},
			},
		}

		got := f.calculateNextInterval(user, &feed, rssFeed, now)
		
		// Expected bootstrap:
		// Sorted chronologically: -15h, -7h, -3h, -1h
		// Gaps: 8h, 4h, 2h
		// Step 1: Initialize ema = 8h
		// Step 2 (gap = 4h): ema = 0.25*4h + 0.75*8h = 7h
		// Step 3 (gap = 2h): ema = 0.25*2h + 0.75*7h = 5.75h = 20700 seconds
		expectedEma := 20700 // 5.75 hours in seconds
		if updatedInterval != expectedEma {
			t.Errorf("expected bootstrapped interval in DB to be %d, got %d", expectedEma, updatedInterval)
		}
		if feed.EstimatedRefreshInterval != expectedEma {
			t.Errorf("expected local feed copy EstimatedRefreshInterval to be updated, got %d", feed.EstimatedRefreshInterval)
		}

		// Silence time: newest is -1h. So T_silent = 1h. Proposed = max(5.75h, 1h) = 5.75h.
		expectedNext := now.Add(5*time.Hour + 45*time.Minute)
		if math.Abs(got.Sub(expectedNext).Seconds()) > 1.0 {
			t.Errorf("expected next fetch at %s, got %s", expectedNext, got)
		}
	})

	t.Run("incremental update for single new item", func(t *testing.T) {
		var updatedInterval int
		db := &storage.MockDB{}
		db.OnUpdateEstimatedRefreshIntervalForFeedForUser = func(u models.User, folderId, id int64, interval int) error {
			updatedInterval = interval
			return nil
		}
		f := Fetcher{d: db}
		
		// Setup feed with prev latest = now - 6h, and current estimate = 4h (14400s)
		feed := models.Feed{
			ID:                       123,
			Latest:                   now.Add(-6 * time.Hour),
			EstimatedRefreshInterval: 14400,
		}

		// New item date is now - 1h (which is after latest of -6h)
		rssFeed := &rss.Feed{
			Refresh: now.Add(10 * time.Minute),
			Items: []*rss.Item{
				{Date: now.Add(-1 * time.Hour), DateValid: true},
			},
		}

		f.calculateNextInterval(user, &feed, rssFeed, now)

		// Expected update:
		// Gap: (now - 1h) - (now - 6h) = 5 hours = 18000s
		// ema_new = 0.25 * 18000 + 0.75 * 14400 = 4500 + 10800 = 15300s (4.25h)
		expectedEma := 15300
		if updatedInterval != expectedEma {
			t.Errorf("expected updated interval in DB to be %d, got %d", expectedEma, updatedInterval)
		}
	})

	t.Run("quiet period silence back-off overrides EMA", func(t *testing.T) {
		db := &storage.MockDB{}
		f := Fetcher{d: db}

		// Feed Latest is 12h ago. Stored EMA is 4h (14400s).
		feed := models.Feed{
			ID:                       123,
			Latest:                   now.Add(-12 * time.Hour),
			EstimatedRefreshInterval: 14400,
		}

		// No new items in fetch
		rssFeed := &rss.Feed{
			Refresh: now.Add(10 * time.Minute),
			Items:   []*rss.Item{},
		}

		got := f.calculateNextInterval(user, &feed, rssFeed, now)

		// T_silent = 12h. Max(4h, 12h) = 12h.
		expectedNext := now.Add(12 * time.Hour)
		if math.Abs(got.Sub(expectedNext).Seconds()) > 1.0 {
			t.Errorf("expected next fetch at %s, got %s", expectedNext, got)
		}
	})
}
