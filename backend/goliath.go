package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/admin"
	"github.com/jrupac/goliath/api"
	"github.com/jrupac/goliath/auth"
	"github.com/jrupac/goliath/cache"
	"github.com/jrupac/goliath/fetch"
	"github.com/jrupac/goliath/opml"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vharitonsky/iniflags"
)

const version = "0.01"

var (
	dbPath       = flag.String("dbPath", "", "The address of the database.")
	port         = flag.Int("port", 9999, "Port of HTTP server.")
	metricsPort  = flag.Int("metricsPort", 9998, "Port to expose Prometheus metrics.")
	publicFolder = flag.String("publicFolder", "public", "Location of static content to serve.")
	// Import/Export options
	opmlUsername   = flag.String("opmlUsername", "", "Username of user to import or export OPML.")
	opmlImportPath = flag.String("opmlImportPath", "", "Path of OPML file to import.")
	opmlExportPath = flag.String("opmlExportPath", "", "Path to file to export OPML.")
)

// Linker-overridden variables.
var buildTimestamp = "<unknown>"
var buildHash = "<unknown>"

func main() {
	iniflags.Parse()
	defer log.Flush()
	ctx := context.Background()

	log.CopyStandardLogTo("INFO")
	log.Infof("Goliath %s.", version)
	t, err := strconv.ParseInt(buildTimestamp, 10, 64)
	if err != nil {
		log.V(2).Infof("Invalid build timestamp %s: %s", buildTimestamp, err)
	} else {
		buildTimestamp = time.Unix(t, 0).String()
	}
	log.Infof("Build time: %s", buildTimestamp)
	log.Infof("Build hash: %s", buildHash)

	if *dbPath == "" {
		log.Fatalf("Path to database must be set.")
	}
	d, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("Unable to open DB: %s", err)
	}
	defer func() {
		err := d.Close()
		if err != nil {
			log.Errorf("Failed to close database: %s", err)
		}
	}()

	processOpml(d)

	ctx, cancel := context.WithCancel(ctx)
	installSignalHandler(cancel)

	retrievalCache, err := cache.StartRetrievalCache(ctx, d)
	if err != nil {
		log.Fatalf("Fatal error while starting retrieval cache: %s", err)
	}

	fetcher := fetch.New(d, retrievalCache)

	go fetcher.Start(ctx)
	go storage.StartGC(ctx, d)
	go admin.Start(ctx, d)
	go serveMetrics(ctx)

	if err = serve(ctx, d); err != nil {
		log.Infof("%s", err)
	}
}

func processOpml(d storage.Database) {
	if *opmlImportPath == "" && *opmlExportPath == "" {
		return
	}

	if *opmlUsername == "" {
		log.Warningf("No OPML username specified.")
		return
	}

	user, err := d.GetUserByUsername(*opmlUsername)
	if err != nil {
		log.Warningf("Error while retrieving user info for OPML: %s", err)
		return
	}

	if *opmlImportPath != "" {
		p, err := opml.ParseOpml(*opmlImportPath)
		if err != nil {
			log.Warningf("Error while parsing OPML: %s", err)
		}
		log.Infof("Completed parsing OPML file %s", *opmlImportPath)
		utils.DebugPrint("Parsed OPML file", *p)

		if err = d.ImportOpmlForUser(user, p); err != nil {
			log.Warningf("Error while importing OPML: %s", err)
		}
	}

	if *opmlExportPath != "" {
		folderTree, err := d.GetFolderFeedTreeForUser(user)
		if err != nil {
			log.Warningf("Error while fetching folder tree: %s", err)
		}

		if err = opml.ExportOpml(folderTree, *opmlExportPath); err != nil {
			log.Warningf("Error while exporting OPML: %s", err)
		} else {
			log.Infof("Completed exporting OPML file to %s", *opmlExportPath)
		}
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

func serveMetrics(ctx context.Context) {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", *metricsPort),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 10,
		Handler:        mux,
	}

	go func(srv *http.Server) {
		<-ctx.Done()
		log.Infof("Shutting down metrics server.")
		if err := srv.Shutdown(ctx); err != nil {
			log.Infof("Failed to cleanly shutdown metrics server: %s", err)
		}
	}(srv)

	mux.Handle("/metrics", promhttp.Handler())
	log.Infof("Starting metrics server on %s", srv.Addr)

	if err := srv.ListenAndServe(); err != nil {
		log.Infof("%s", err)
	}
}

func serve(ctx context.Context, d storage.Database) error {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", *port),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 10,
		Handler:        mux,
	}

	go func(srv *http.Server) {
		<-ctx.Done()
		log.Infof("Shutting down HTTP server.")
		if err := srv.Shutdown(ctx); err != nil {
			log.Infof("Failed to cleanly shutdown HTTP server: %s", err)
		}
	}(srv)

	mux.HandleFunc("/auth", auth.HandleLogin(d))
	mux.HandleFunc("/logout", auth.HandleLogout)
	mux.HandleFunc("/fever/", api.FeverHandler(d))
	mux.HandleFunc("/greader/", api.GReaderHandler(d))
	mux.HandleFunc("/version", handleVersion)
	mux.Handle("/cache", auth.WithAuth(cache.NewImageProxy(), d, *publicFolder, cache.AuthErrorRedirect, true))
	mux.Handle("/static/", http.FileServer(http.Dir(*publicFolder)))
	mux.Handle("/", auth.WithAuth(http.FileServer(http.Dir(*publicFolder)), d, *publicFolder, nil, false))
	log.Infof("Starting HTTP server on %s", srv.Addr)
	return srv.ListenAndServe()
}

func handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]string{
		"build_timestamp": buildTimestamp,
		"build_hash":      buildHash,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Warningf("Failed to encode response JSON: %s", err)
	}
}
