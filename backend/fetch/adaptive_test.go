package fetch

import (
	"math"
	"testing"
	"time"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/rss"
)

func TestCalculateAdaptiveInterval(t *testing.T) {
	// Temporarily override flags to ensure tests are deterministic
	oldMin := *minFetchInterval
	oldMax := *maxFetchInterval
	*minFetchInterval = 10 * time.Minute
	*maxFetchInterval = 24 * time.Hour
	defer func() {
		*minFetchInterval = oldMin
		*maxFetchInterval = oldMax
	}()

	now := time.Now()

	tests := []struct {
		name     string
		articles []models.Article
		expected time.Duration
	}{
		{
			name:     "fewer than 2 articles",
			articles: []models.Article{{Date: now.Add(-1 * time.Hour)}},
			expected: 10 * time.Minute,
		},
		{
			name: "duplicate timestamps (less than 2 unique)",
			articles: []models.Article{
				{Date: now.Add(-1 * time.Hour)},
				{Date: now.Add(-1 * time.Hour)},
			},
			expected: 10 * time.Minute,
		},
		{
			name: "correct EMA gap calculation (active)",
			articles: []models.Article{
				{Date: now.Add(-1 * time.Hour)},
				{Date: now.Add(-3 * time.Hour)},
				{Date: now.Add(-7 * time.Hour)},
				{Date: now.Add(-15 * time.Hour)},
			},
			// Gaps: 2h, 4h, 8h
			// S_3 = 8h
			// S_2 = 0.25*4h + 0.75*8h = 7h
			// S_1 = 0.25*2h + 0.75*7h = 5.75h (5h45m)
			// Silent time = 1h. Max(5.75h, 1h) = 5.75h.
			expected: 5*time.Hour + 45*time.Minute,
		},
		{
			name: "quiet period back-off",
			articles: []models.Article{
				{Date: now.Add(-12 * time.Hour)},
				{Date: now.Add(-13 * time.Hour)},
				{Date: now.Add(-14 * time.Hour)},
				{Date: now.Add(-15 * time.Hour)},
			},
			// Gaps: 1h, 1h, 1h
			// S_1 = 1h
			// Silent time = 12h. Max(1h, 12h) = 12h.
			expected: 12 * time.Hour,
		},
		{
			name: "max clamping",
			articles: []models.Article{
				{Date: now.Add(-48 * time.Hour)},
				{Date: now.Add(-49 * time.Hour)},
				{Date: now.Add(-50 * time.Hour)},
			},
			// EMA = 1h. Silent = 48h. Clamped to Max flag (24h).
			expected: 24 * time.Hour,
		},
		{
			name: "filters out future-dated articles",
			articles: []models.Article{
				{Date: now.Add(2 * time.Hour)}, // Future
				{Date: now.Add(-1 * time.Hour)},
				{Date: now.Add(-3 * time.Hour)},
				{Date: now.Add(-7 * time.Hour)},
				{Date: now.Add(-15 * time.Hour)},
			},
			expected: 5*time.Hour + 45*time.Minute,
		},
		{
			name: "filters out zero dates",
			articles: []models.Article{
				{Date: time.Time{}},
				{Date: now.Add(-1 * time.Hour)},
				{Date: now.Add(-3 * time.Hour)},
				{Date: now.Add(-7 * time.Hour)},
				{Date: now.Add(-15 * time.Hour)},
			},
			expected: 5*time.Hour + 45*time.Minute,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := calculateAdaptiveInterval(now, tc.articles)
			if got != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}

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
	feed := models.Feed{ID: 123}

	t.Run("respects custom non-10m refresh directly", func(t *testing.T) {
		db := &storage.MockDB{}
		f := Fetcher{d: db}

		customRefresh := now.Add(3 * time.Hour)
		rssFeed := &rss.Feed{
			Refresh: customRefresh,
		}

		got := f.calculateNextInterval(user, feed, rssFeed, now)
		if !got.Equal(customRefresh) {
			t.Errorf("expected custom refresh %s to be respected, got %s", customRefresh, got)
		}
	})

	t.Run("calculates adaptive interval on default 10m refresh", func(t *testing.T) {
		db := &storage.MockDB{
			OnGetArticlesForFeedForUser: func(u models.User, id int64) ([]models.Article, error) {
				return []models.Article{
					{Date: now.Add(-1 * time.Hour)},
					{Date: now.Add(-3 * time.Hour)},
					{Date: now.Add(-7 * time.Hour)},
					{Date: now.Add(-15 * time.Hour)},
				}, nil
			},
		}
		f := Fetcher{d: db}

		// Exact 10m duration from now
		rssFeed := &rss.Feed{
			Refresh: now.Add(10 * time.Minute),
		}

		got := f.calculateNextInterval(user, feed, rssFeed, now)
		expectedNext := now.Add(5*time.Hour + 45*time.Minute)
		if math.Abs(got.Sub(expectedNext).Seconds()) > 1.0 {
			t.Errorf("expected adaptive interval to resolve to %s, got %s", expectedNext, got)
		}
	})
}
