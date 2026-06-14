package api

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// readActivityLabels returns the current hour_of_day (zero-padded "00"–"23")
// and day_of_week ("1-Mon" through "7-Sun", ISO order) for use as metric labels.
func readActivityLabels() (hourOfDay, dayOfWeek string) {
	now := time.Now()
	wd := now.Weekday()
	isoDay := int(wd)
	if isoDay == 0 {
		isoDay = 7
	}
	return fmt.Sprintf("%02d", now.Hour()), fmt.Sprintf("%d-%s", isoDay, wd.String()[:3])
}

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
)

func init() {
	prometheus.MustRegister(articlesMarkedReadMetric)
	prometheus.MustRegister(articlesSavedMetric)
}
