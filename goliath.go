package main

import (
	"encoding/json"
	"flag"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/opml"
	"github.com/jrupac/goliath/storage"
)

const VERSION = "0.01"

var (
	dbPath = flag.String("dbPath", "", "The address of the database.")
)

func main() {
	flag.Parse()
	defer log.Flush()

	log.Infof("Goliath %s.", VERSION)

	d, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("Unable to open DB: %s", err)
	}
	defer d.Close()

	p, err := opml.ParseOpml("testdata/opml2.xml")
	if err != nil {
		log.Warningf("Error while parsing OPML: %s", err)
	}

	b, err := json.MarshalIndent(*p, "", " ")
	log.Infof("Parsed OPML file: %s\n", string(b))

	err = d.ImportOpml(p)
	if err != nil {
		log.Warningf("Error while importing OPML: %s", err)
	}
}
