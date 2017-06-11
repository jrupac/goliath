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
	"encoding/json"
	"strconv"
)

const VERSION = "0.01"

var (
	dbPath   = flag.String("dbPath", "", "The address of the database.")
	opmlPath = flag.String("opmlPath", "", "Path of OPML file to import.")
	port = flag.Int("port", 9999, "Port of HTTP server.")
	publicFolder = flag.String("publicFolder", "public", "Location of static content to serve.")
)

// Linker-initialized compile-time variables.
var buildTimestamp string
var buildHash string

func main() {
	flag.Parse()
	defer log.Flush()
	ctx := context.Background()

	log.CopyStandardLogTo("INFO")
	log.Infof("Goliath %s.", VERSION)
	t, err := strconv.ParseInt(buildTimestamp, 10, 64)
	if err != nil {
		log.Warningf("Invalid build timestamp %s: %s", buildTimestamp, err)
	} else {
		buildTimestamp = time.Unix(t, 0).String()
		log.Infof("Built at: %s", buildTimestamp)
	}
	log.Infof("Build hash: %s", buildHash)

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
		log.Infof("Completed parsing OPML file %s", *opmlPath)
		utils.DebugPrint("Parsed OPML file", *p)

		err = d.ImportOpml(p)
		if err != nil {
			log.Warningf("Error while importing OPML: %s", err)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	installSignalHandler(cancel)

	go fetch.Start(ctx, d)
	go storage.StartGc(ctx, d)

	if err = Serve(ctx, d); err != nil {
		log.Infof("%s", err)
	}
}

func installSignalHandler(cancel context.CancelFunc) {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		s := <-sc
		log.Infof("Received signal and shutting down: %s", s)
		close(sc)
		cancel()
	}()
}

func Serve(ctx context.Context, d *storage.Database) error {
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", *port),
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
		MaxHeaderBytes: 1 << 10,
	}

	go func(srv *http.Server) {
		select {
		case <- ctx.Done():
			log.Infof("Shutting down HTTP server.")
			if err := srv.Shutdown(nil); err != nil {
				log.Infof("Failed to cleanly shutdown HTTP server: %s", err)
			}
		}
	}(srv)

	http.HandleFunc("/auth", auth.HandleLogin(d));
	http.HandleFunc("/logout", auth.HandleLogout);
	http.HandleFunc("/fever/", HandleFever(d))
	http.HandleFunc("/version", HandleVersion)
	http.Handle("/static/", http.FileServer(http.Dir(*publicFolder)))
	http.Handle("/", auth.WithAuth(http.FileServer(http.Dir(*publicFolder)), d, *publicFolder))
	log.Infof("Starting HTTP server on port %d", *port)
	return srv.ListenAndServe()
}

func HandleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]string{
		"build_timestamp": buildTimestamp,
		"build_hash": buildHash,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Warningf("Failed to encode response JSON: %s", err)
	}
}