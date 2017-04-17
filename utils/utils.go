package utils

import (
	"encoding/json"
	log "github.com/golang/glog"
)

func DebugPrint(prefix string, s interface{}) {
	b, _ := json.MarshalIndent(s, "", " ")
	log.V(2).Infof("%s: %s\n", prefix, string(b))
}
