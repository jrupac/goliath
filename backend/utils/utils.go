package utils

import (
	"encoding/json"
	log "github.com/golang/glog"
	"net/http"
	"net/http/httputil"
	"time"
)

// DebugPrint pretty-prints the given object with the given prefix at v=2 verbosity.
func DebugPrint(prefix string, s interface{}) {
	b, _ := json.MarshalIndent(s, "", " ")
	log.V(2).Infof("%s: %s\n", prefix, string(b))
}

// Elapsed calls the specified callback and passed the elapsed duration since the given start time.
func Elapsed(start time.Time, callback func(time.Duration)) {
	elapsed := time.Since(start)
	callback(elapsed)
}

// HttpRequestPrint pretty-prints a given http.Request object.
func HttpRequestPrint(prefix string, r *http.Request) {
	b, _ := httputil.DumpRequest(r, true)
	log.Infof("%s:\n %s", prefix, string(b))
}

// Substring returns a substring at most n characters long.
func Substring(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s
}
