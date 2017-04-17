package main

import (
	"context"
	"flag"
	"fmt"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/fetch"
	"github.com/jrupac/goliath/opml"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

const VERSION = "0.01"

var (
	dbPath   = flag.String("dbPath", "", "The address of the database.")
	opmlPath = flag.String("opmlPath", "", "Path of OPML file to import.")
	portFlag = flag.Int("port", 9999, "Port of HTTP server.")
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

	go fetch.Do(ctx, d, allFeeds)
	srv := &http.Server{Addr: fmt.Sprintf(":%d", *portFlag)}
	installSignalHandler(cancel, srv)

	if err = Serve(srv); err != nil {
		log.Infof("%s", err)
	}
}

func installSignalHandler(cancel context.CancelFunc, srv *http.Server) {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		s := <-sc
		log.Infof("Received signal and shutting down: %s", s)
		cancel()
		log.Infof("Shutting down HTTP server.")
		if err := srv.Shutdown(nil); err != nil {
			log.Infof("Failed to cleanly shutdown HTTP server: %s", err)
		}
	}()
}

func Serve(srv *http.Server) error {
	http.HandleFunc("/fever", HandleFever)
	log.Infof("Starting HTTP server on port %d", *portFlag)
	return srv.ListenAndServe()
}
