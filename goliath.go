package main

import (
	"flag"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/opml"
)

const VERSION = "0.01"

func main() {
	flag.Parse()
	defer log.Flush()

	log.Infof("Goliath %s.", VERSION)

	p, err := opml.ParseOpml("testdata/opml2.xml")
	if err != nil {
		log.Warningf("Error while parsing OPML: %s", err)
	}

	log.Infof("Parsed OPML file: %+v", *p)
}
