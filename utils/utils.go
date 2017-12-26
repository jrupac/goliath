package utils

import (
	"encoding/json"
	log "github.com/golang/glog"
)

// DebugPrint pretty-prints the given object with the given prefix at v=2 verbosity.
func DebugPrint(prefix string, s interface{}) {
	b, _ := json.MarshalIndent(s, "", " ")
	log.V(2).Infof("%s: %s\n", prefix, string(b))
}
