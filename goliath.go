package main

import (
	"flag"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/db"
	"github.com/jrupac/goliath/opml"
)

const VERSION = "0.01"

var (
	dbPath = flag.String("dbPath", "", "The address of the database.")
)

func main() {
	flag.Parse()
	defer log.Flush()

	log.Infof("Goliath %s.", VERSION)

	d, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("Unable to open DB: %s", err)
	}
	defer d.Close()

	p, err := opml.ParseOpml("testdata/opml2.xml")
	if err != nil {
		log.Warningf("Error while parsing OPML: %s", err)
	}

	log.Infof("Parsed OPML file: %+v", *p)
}
