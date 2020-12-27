package api

import (
	"encoding/json"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"time"
)

var (
	freshrssLatencyMetric = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "freshrss_server_latency",
			Help:       "Server-side latency of FreshRSS API operations.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"method"},
	)
)

func init() {
	prometheus.MustRegister(freshrssLatencyMetric)
}

// FreshRSS is an implementation of the FreshRSS API.
type FreshRSS struct {
}

// FreshRSSHandler returns a new FressRSS handler.
func FreshRSSHandler(d *storage.Database) http.HandlerFunc {
	return FreshRSS{}.Handler(d)
}

// Handler returns a handler function that implements the FreshRSS API.
func (a FreshRSS) Handler(d *storage.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.handle(d, w, r)
	}
}

func (a FreshRSS) recordLatency(t time.Time, label string) {
	utils.Elapsed(t, func(d time.Duration) {
		// Record latency measurements in microseconds.
		freshrssLatencyMetric.WithLabelValues(label).Observe(float64(d) / float64(time.Microsecond))
	})
}

func (a FreshRSS) handle(_ *storage.Database, w http.ResponseWriter, r *http.Request) {
	// Record the total server latency of each call.
	defer a.recordLatency(time.Now(), "server")

	resp := apiResponse{}

	err := r.ParseForm()
	if err != nil {
		a.returnError(w, "Failed to parse request: %s", err)
		return
	}

	log.Infof("FreshRSS request URL: %s", r.URL.String())
	log.Infof("FreshRSS request body: %s", r.PostForm.Encode())

	w.Header().Set("Content-Type", "application/json")
	a.returnSuccess(w, resp)
}

func (a FreshRSS) returnError(w http.ResponseWriter, msg string, err error) {
	log.Warningf(msg, err)
	if fe, ok := err.(*apiError); ok {
		if fe.internal {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}

func (a FreshRSS) returnSuccess(w http.ResponseWriter, resp apiResponse) {
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	// HTML content is escaped already during fetch time (e.g., ' --> &#39;), so
	// do not HTML escape it further (e.g., &#39; --> \u0026#39;), which would
	// render incorrectly on a webpage.
	enc.SetEscapeHTML(false)
	if err := enc.Encode(resp); err != nil {
		a.returnError(w, "Failed to encode response JSON: %s", err)
	}
}
