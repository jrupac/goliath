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
	"github.com/jrupac/goliath/auth"
	"time"
)

const VERSION = "0.01"

var (
	dbPath   = flag.String("dbPath", "", "The address of the database.")
	opmlPath = flag.String("opmlPath", "", "Path of OPML file to import.")
	portFlag = flag.Int("port", 9999, "Port of HTTP server.")
	publicFolder = flag.String("publicFolder", "public", "Location of static content to serve.")
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

	go fetch.Start(ctx, d, allFeeds)
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", *portFlag),
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
		MaxHeaderBytes: 1 << 10,
	}
	installSignalHandler(cancel, srv)

	if err = Serve(srv, d); err != nil {
		log.Infof("%s", err)
	}
}

func installSignalHandler(cancel context.CancelFunc, srv *http.Server) {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		s := <-sc
		log.Infof("Received signal and shutting down: %s", s)
		close(sc)
		cancel()
		log.Infof("Shutting down HTTP server.")
		if err := srv.Shutdown(nil); err != nil {
			log.Infof("Failed to cleanly shutdown HTTP server: %s", err)
		}
	}()
}

func Serve(srv *http.Server, d *storage.Database) error {
	http.HandleFunc("/auth", auth.HandleLogin(d));
	http.HandleFunc("/logout", auth.HandleLogout);
	http.HandleFunc("/fever/", HandleFever(d))
	http.Handle("/static/", http.FileServer(http.Dir(*publicFolder)))
	http.Handle("/", auth.WithAuth(http.FileServer(http.Dir(*publicFolder)), d, *publicFolder))
	log.Infof("Starting HTTP server on port %d", *portFlag)
	return srv.ListenAndServe()
}