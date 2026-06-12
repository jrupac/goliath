package api

import "github.com/prometheus/client_golang/prometheus"

var (
	articlesMarkedReadMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "articles_marked_read_total",
			Help: "Total number of mark-read events. 'scope' is 'individual' for per-article marks via edit-tag, 'feed' or 'folder' for bulk mark-all-as-read events (counts mark operations, not articles).",
		},
		[]string{"username", "scope"},
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
