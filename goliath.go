package main

import (
	"context"
	"flag"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/fetch"
	"github.com/jrupac/goliath/opml"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"os"
	"os/signal"
	"syscall"
)

const VERSION = "0.01"

var (
	dbPath   = flag.String("dbPath", "", "The address of the database.")
	opmlPath = flag.String("opmlPath", "", "Path of OPML file to import.")
)

func main() {
	flag.Parse()
	defer log.Flush()
	ctx := context.Background()

	log.Infof("Goliath %s.", VERSION)

	if *dbPath == "" {
		log.Fatalf("Path to database must be set.")
	}
	d, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("Unable to open DB: %s", err)
	}
	defer d.Close()

	if *opmlPath != "" {
		p, err := opml.ParseOpml(*opmlPath)
		if err != nil {
			log.Warningf("Error while parsing OPML: %s", err)
		}
		utils.DebugPrint("Parsed OPML file", *p)

		err = d.ImportOpml(p)
		if err != nil {
			log.Warningf("Error while importing OPML: %s", err)
		}
	}

	allFeeds, err := d.GetAllFeeds()
	if err != nil {
		log.Infof("Failed to fetch all feeds: %s", err)
	}
	utils.DebugPrint("Feed list", allFeeds)
	ctx, cancel := context.WithCancel(ctx)
	installSignalHandler(cancel)

	fetch.Do(ctx, d, allFeeds)
}

func installSignalHandler(cancel context.CancelFunc) {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		s := <-sc
		log.Infof("Received signal and shutting down: %s", s)
		cancel()
	}()
}
