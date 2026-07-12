package api

import (
	"fmt"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	articlesMarkedReadMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "articles_marked_read_total",
			Help: "Total number of mark-read events. 'scope' is 'individual' for per-article marks via edit-tag, 'feed' or 'folder' for bulk mark-all-as-read events (counts mark operations, not articles). 'hour_of_day' is zero-padded 00–23; 'day_of_week' is ISO weekday prefixed 1-Mon through 7-Sun.",
		},
		[]string{"username", "scope", "hour_of_day", "day_of_week"},
	)
	articlesSavedMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "articles_saved_total",
			Help: "Total number of articles saved/starred by the user.",
		},
		[]string{"username"},
	)

	initializedUsers sync.Map
	easternLocation  *time.Location
)

// activityLabelsForTime returns the hour_of_day (zero-padded "00"–"23")
// and day_of_week ("1-Mon" through "7-Sun", ISO order) for a specific time,
// converted to America/New_York timezone.
func activityLabelsForTime(t time.Time) (hourOfDay, dayOfWeek string) {
	t = t.In(easternLocation)
	wd := t.Weekday()
	isoDay := int(wd)
	if isoDay == 0 {
		isoDay = 7
	}
	return fmt.Sprintf("%02d", t.Hour()), fmt.Sprintf("%d-%s", isoDay, wd.String()[:3])
}

// readActivityLabels returns the current hour_of_day (zero-padded "00"–"23")
// and day_of_week ("1-Mon" through "7-Sun", ISO order) for use as metric labels.
func readActivityLabels() (hourOfDay, dayOfWeek string) {
	return activityLabelsForTime(time.Now())
}

// InitUserMetrics pre-initializes the read activity metrics for a given user
// with 0 values for all possible combinations of scope, hour, and day.
// This ensures that Prometheus scrapes these series and they show up as 0
// instead of being absent in Grafana.
func InitUserMetrics(username string) {
	if _, loaded := initializedUsers.LoadOrStore(username, true); loaded {
		return
	}

	scopes := []string{"individual", "feed", "folder"}
	// 2023-01-02 was a Monday, used as a reference to generate all weekdays (Monday-Sunday)
	baseTime := time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)

	for _, scope := range scopes {
		for d := 0; d < 7; d++ {
			dayTime := baseTime.AddDate(0, 0, d)
			for h := 0; h < 24; h++ {
				t := dayTime.Add(time.Duration(h) * time.Hour)
				hourStr, dayStr := activityLabelsForTime(t)
				articlesMarkedReadMetric.WithLabelValues(username, scope, hourStr, dayStr).Add(0)
			}
		}
	}
}

func init() {
	var err error
	easternLocation, err = time.LoadLocation("America/New_York")
	if err != nil {
		// Fallback to static EST (-5) if America/New_York fails to load
		easternLocation = time.FixedZone("EST", -5*60*60)
	}

	prometheus.MustRegister(articlesMarkedReadMetric)
	prometheus.MustRegister(articlesSavedMetric)
}
