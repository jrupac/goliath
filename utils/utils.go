package utils

import (
	"encoding/json"
	"flag"
	log "github.com/golang/glog"
)

var (
	debugFlag = flag.Bool("debug", false, "True to enable additional debug logging.")
)

func DebugPrint(prefix string, s interface{}) {
	if !*debugFlag {
		return
	}

	b, _ := json.MarshalIndent(s, "", " ")
	log.Infof("%s: %s\n", prefix, string(b))
}
