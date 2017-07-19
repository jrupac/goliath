package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/auth"
	"github.com/jrupac/goliath/fetch"
	"github.com/jrupac/goliath/opml"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const VERSION = "0.01"

var (
	dbPath       = flag.String("dbPath", "", "The address of the database.")
	opmlPath     = flag.String("opmlPath", "", "Path of OPML file to import.")
	port         = flag.Int("port", 9999, "Port of HTTP server.")
	metricsPort  = flag.Int("metricsPort", 9998, "Port to expose Prometheus metrics.")
	publicFolder = flag.String("publicFolder", "public", "Location of static content to serve.")
)

// Linker-overridden variables.
var buildTimestamp = "<unknown>"
var buildHash = "<unknown>"

func main() {
	flag.Parse()
	defer log.Flush()
	ctx := context.Background()

	log.CopyStandardLogTo("INFO")
	log.Infof("Goliath %s.", VERSION)
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

	go func() {
		if err = ServeMetrics(ctx); err != nil {
			log.Infof("%s", err)
		}
	}()

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

func ServeMetrics(ctx context.Context) error {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", *metricsPort),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 10,
		Handler:        mux,
	}

	go func(srv *http.Server) {
		select {
		case <-ctx.Done():
			log.Infof("Shutting down metrics server.")
			if err := srv.Shutdown(nil); err != nil {
				log.Infof("Failed to cleanly shutdown metrics server: %s", err)
			}
		}
	}(srv)

	mux.Handle("/metrics", promhttp.Handler())
	log.Infof("Starting metrics server on %s", srv.Addr)
	return srv.ListenAndServe()
}

func Serve(ctx context.Context, d *storage.Database) error {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", *port),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 10,
		Handler:        mux,
	}

	go func(srv *http.Server) {
		select {
		case <-ctx.Done():
			log.Infof("Shutting down HTTP server.")
			if err := srv.Shutdown(nil); err != nil {
				log.Infof("Failed to cleanly shutdown HTTP server: %s", err)
			}
		}
	}(srv)

	mux.HandleFunc("/auth", auth.HandleLogin(d))
	mux.HandleFunc("/logout", auth.HandleLogout)
	mux.HandleFunc("/fever/", HandleFever(d))
	mux.HandleFunc("/version", HandleVersion)
	mux.Handle("/static/", http.FileServer(http.Dir(*publicFolder)))
	mux.Handle("/", auth.WithAuth(http.FileServer(http.Dir(*publicFolder)), d, *publicFolder))
	log.Infof("Starting HTTP server on %s", srv.Addr)
	return srv.ListenAndServe()
}

func HandleVersion(w http.ResponseWriter, _ *http.Request) {
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
