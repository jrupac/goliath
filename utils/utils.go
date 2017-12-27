package utils

import (
	"encoding/json"
	log "github.com/golang/glog"
	"time"
)

// DebugPrint pretty-prints the given object with the given prefix at v=2 verbosity.
func DebugPrint(prefix string, s interface{}) {
	b, _ := json.MarshalIndent(s, "", " ")
	log.V(2).Infof("%s: %s\n", prefix, string(b))
}

// Elapsed calls the the specified callback and passed the elapsed duration since the given start time.
func Elapsed(start time.Time, callback func(time.Duration)) {
	elapsed := time.Since(start)
	callback(elapsed)
}
