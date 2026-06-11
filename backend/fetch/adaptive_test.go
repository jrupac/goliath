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
	oldAlphaFaster := *emaAlphaFaster
	oldAlphaSlower := *emaAlphaSlower
	oldMaxGapMultiple := *maxGapEMAMultiple
	*minFetchInterval = 10 * time.Minute
	*maxFetchInterval = 24 * time.Hour
	*emaAlphaFaster = 0.5
	*emaAlphaSlower = 0.1
	*maxGapEMAMultiple = 3.0
	defer func() {
		*minFetchInterval = oldMin
		*maxFetchInterval = oldMax
		*emaAlphaFaster = oldAlphaFaster
		*emaAlphaSlower = oldAlphaSlower
		*maxGapEMAMultiple = oldMaxGapMultiple
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

		// Expected bootstrap (sorted oldest first: -15h, -7h, -3h, -1h):
		// Step 1: ema = 8h (first gap initialised directly, no alpha)
		// Step 2 (gap=4h < ema=8h): alphaFaster=0.5 → ema = 0.5*4h + 0.5*8h = 6h
		// Step 3 (gap=2h < ema=6h): alphaFaster=0.5 → ema = 0.5*2h + 0.5*6h = 4h = 14400s
		expectedEma := 14400
		if updatedInterval != expectedEma {
			t.Errorf("expected bootstrapped interval in DB to be %d, got %d", expectedEma, updatedInterval)
		}
		if feed.EstimatedRefreshInterval != expectedEma {
			t.Errorf("expected local feed copy EstimatedRefreshInterval to be updated, got %d", feed.EstimatedRefreshInterval)
		}

		// Next fetch is EMA = 4h with no silence penalty.
		expectedNext := now.Add(4 * time.Hour)
		if math.Abs(got.Sub(expectedNext).Seconds()) > 1.0 {
			t.Errorf("expected next fetch at %s, got %s", expectedNext, got)
		}
	})

	t.Run("incremental update for single new item (gap > EMA uses slower alpha)", func(t *testing.T) {
		var updatedInterval int
		db := &storage.MockDB{}
		db.OnUpdateEstimatedRefreshIntervalForFeedForUser = func(u models.User, folderId, id int64, interval int) error {
			updatedInterval = interval
			return nil
		}
		f := Fetcher{d: db}

		// Feed with prev latest = now-6h, current estimate = 4h (14400s)
		feed := models.Feed{
			ID:                       123,
			Latest:                   now.Add(-6 * time.Hour),
			EstimatedRefreshInterval: 14400,
		}

		// New item at now-1h: gap0 = (now-1h) - (now-6h) = 5h
		rssFeed := &rss.Feed{
			Refresh: now.Add(10 * time.Minute),
			Items: []*rss.Item{
				{Date: now.Add(-1 * time.Hour), DateValid: true},
			},
		}

		f.calculateNextInterval(user, &feed, rssFeed, now)

		// gap0=5h > EMA=4h → alphaSlower=0.1; cap=12h (not hit)
		// ema = 0.1*5h + 0.9*4h = 0.5h + 3.6h = 4.1h = 14760s
		expectedEma := 14760
		if updatedInterval != expectedEma {
			t.Errorf("expected updated interval in DB to be %d, got %d", expectedEma, updatedInterval)
		}
	})

	t.Run("no silence penalty when feed has known Latest and no new items", func(t *testing.T) {
		db := &storage.MockDB{}
		f := Fetcher{d: db}

		// Feed Latest is 12h ago, stored EMA is 4h (14400s).
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

		// EMA unchanged (no new items). No silence penalty → interval = EMA = 4h.
		expectedNext := now.Add(4 * time.Hour)
		if math.Abs(got.Sub(expectedNext).Seconds()) > 1.0 {
			t.Errorf("expected next fetch at %s, got %s", expectedNext, got)
		}
	})

	t.Run("gap below EMA uses faster alpha", func(t *testing.T) {
		var updatedInterval int
		db := &storage.MockDB{}
		db.OnUpdateEstimatedRefreshIntervalForFeedForUser = func(u models.User, folderId, id int64, interval int) error {
			updatedInterval = interval
			return nil
		}
		f := Fetcher{d: db}

		// Feed with current EMA = 2h (7200s), last article 3h ago.
		feed := models.Feed{
			ID:                       123,
			Latest:                   now.Add(-3 * time.Hour),
			EstimatedRefreshInterval: 7200,
		}

		// New item at now-2h30m: gap0 = 30m, well below EMA of 2h.
		rssFeed := &rss.Feed{
			Refresh: now.Add(10 * time.Minute),
			Items: []*rss.Item{
				{Date: now.Add(-150 * time.Minute), DateValid: true},
			},
		}

		f.calculateNextInterval(user, &feed, rssFeed, now)

		// gap0=30m < EMA=2h → alphaFaster=0.5; cap=6h (not hit)
		// ema = 0.5*30m + 0.5*2h = 15m + 1h = 75m = 4500s
		expectedEma := 4500
		if updatedInterval != expectedEma {
			t.Errorf("expected updated interval in DB to be %d, got %d", expectedEma, updatedInterval)
		}
	})

	t.Run("gap cap prevents large outlier gaps from inflating EMA", func(t *testing.T) {
		var updatedInterval int
		db := &storage.MockDB{}
		db.OnUpdateEstimatedRefreshIntervalForFeedForUser = func(u models.User, folderId, id int64, interval int) error {
			updatedInterval = interval
			return nil
		}
		f := Fetcher{d: db}

		// Feed with current EMA = 20m (1200s). Last article was 10h ago.
		feed := models.Feed{
			ID:                       123,
			Latest:                   now.Add(-10 * time.Hour),
			EstimatedRefreshInterval: 1200,
		}

		// New item at now-30m: gap0 = 9h30m = 570m, far exceeds EMA.
		rssFeed := &rss.Feed{
			Refresh: now.Add(10 * time.Minute),
			Items: []*rss.Item{
				{Date: now.Add(-30 * time.Minute), DateValid: true},
			},
		}

		f.calculateNextInterval(user, &feed, rssFeed, now)

		// gap0=570m capped at 20m*3.0 = 60m; 60m > 20m → alphaSlower=0.1
		// ema = 0.1*60m + 0.9*20m = 6m + 18m = 24m = 1440s
		// (without cap: 0.1*570m + 0.9*20m = 57m + 18m = 75m = 4500s)
		expectedEma := 1440
		if updatedInterval != expectedEma {
			t.Errorf("expected capped interval in DB to be %d, got %d", expectedEma, updatedInterval)
		}
	})
}
