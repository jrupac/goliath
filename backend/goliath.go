package main

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/jrupac/goliath/schema"
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
	// Only for use in unusual cases when running a binary against a different schema version.
	skipSchemaCheck = flag.Bool("skipSchemaCheck", false,
		"Start even if the database's schema version says this binary cannot run on it.")
)

// Linker-overridden variables.
var buildTimestamp = "<unknown>"
var buildHash = "<unknown>"

// dbSchemaVersion is the version the database was at when the server started.
// Only goliath-cli migrates, and it stops the server to do so.
var dbSchemaVersion int

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

	checkSchema(d)
	processOpml(d)

	users, err := d.GetAllUsers()
	if err != nil {
		log.Warningf("Failed to query users to initialize metrics: %s", err)
	} else {
		for _, u := range users {
			api.InitUserMetrics(u.Username)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	installSignalHandler(cancel)

	retrievalCache, err := cache.StartRetrievalCache(d)
	if err != nil {
		log.Fatalf("Fatal error while starting retrieval cache: %s", err)
	}

	feedAllowlist, err := fetch.NewFeedAllowlist()
	if err != nil {
		// Fatal rather than fetching with an empty allowlist: starting anyway
		// would refuse the very feeds the allowlist was written to permit, and
		// report it as those feeds failing.
		log.Fatalf("Invalid feed address allowlist: %s", err)
	}

	fetcher, err := fetch.New(d, retrievalCache, feedAllowlist)
	if err != nil {
		log.Fatalf("Invalid feed fetcher configuration: %s", err)
	}
	scheduler := fetch.NewScheduler(fetcher, d)

	fetching := make(chan struct{})
	go func() {
		defer close(fetching)
		scheduler.Run(ctx)
	}()
	go storage.StartGC(ctx, d)
	go admin.Start(ctx, d, scheduler)
	go serveMetrics(ctx)

	api.InitPostTokenKey()

	if err = cache.CheckImageProxyKey(fetch.ImageProxyingEnabled()); err != nil {
		// Fatal rather than disabling proxying: silently serving unproxied
		// images looks like the feature working badly, whereas refusing to
		// start names the thing that needs fixing.
		log.Fatalf("Invalid image proxy configuration: %s", err)
	}

	if err = serve(ctx, d, scheduler); err != nil {
		log.Infof("%s", err)
	}
	// A signal is what normally stops the server; if anything else did, the
	// rest has to be told.
	cancel()
	shutDown(fetching, retrievalCache)
}

const (
	// httpDrainTimeout bounds how long an HTTP server waits for requests in
	// flight once told to stop.
	httpDrainTimeout = 5 * time.Second
	// fetchStopTimeout bounds how long shutting down then waits for fetches
	// still in flight. With the drain and the retrieval cache's last write, it
	// has to fit in the time a container is given to stop before it is killed.
	fetchStopTimeout = 3 * time.Second
)

// shutDown stops what writes to the database, in order, before main closes it.
//
// Fetching stops first, since the retrieval cache's last write would otherwise
// miss what the last fetches added to it. Fetching has been stopping since the
// signal, alongside the HTTP drain, so this usually does not wait at all.
func shutDown(fetching <-chan struct{}, retrievalCache *cache.CuckooFilterRetrievalCache) {
	select {
	case <-fetching:
	case <-time.After(fetchStopTimeout):
		log.Warningf("Fetching has not stopped after %s; writing the retrieval cache regardless.", fetchStopTimeout)
	}
	retrievalCache.Close()
	log.Infof("Stopped; closing the database.")
}

// shutdownServer stops srv, giving the requests in flight httpDrainTimeout to
// finish. It takes a fresh context: the one the process stops on is already
// cancelled, and Shutdown given that returns without waiting for any.
func shutdownServer(srv *http.Server, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), httpDrainTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Infof("Failed to cleanly shut down %s: %s", name, err)
	}
}

// checkSchema refuses to start against a database this binary cannot use.
// This can be overridden by the `skipSchemaCheck` flag.
func checkSchema(d storage.Database) {
	applied, err := d.GetSchemaVersions()
	if err != nil {
		log.Fatalf("Unable to read the database's schema version: %s", err)
	}
	dbSchemaVersion = schema.Current(applied)
	log.Infof("Schema: binary v%d, database v%d", schema.Latest(), dbSchemaVersion)

	if err = schema.Check(schema.Latest(), applied); err != nil {
		if !*skipSchemaCheck {
			// A refusal, not a crash, so no stack trace.
			log.Exitf("Incompatible schema: %s", err)
		}
		log.Warningf("Starting despite an incompatible schema, as asked: %s", err)
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

	go func() {
		<-ctx.Done()
		log.Infof("Shutting down metrics server.")
		shutdownServer(srv, "metrics server")
	}()

	mux.Handle("/metrics", promhttp.Handler())
	log.Infof("Starting metrics server on %s", srv.Addr)

	if err := srv.ListenAndServe(); err != nil {
		log.Infof("%s", err)
	}
}

func serve(ctx context.Context, d storage.Database, subs fetch.Subscriptions) error {
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", *port),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 10,
		Handler:        newMux(d, subs),
	}

	drained := make(chan struct{})
	go func() {
		defer close(drained)
		<-ctx.Done()
		log.Infof("Shutting down HTTP server.")
		shutdownServer(srv, "HTTP server")
	}()

	log.Infof("Starting HTTP server on %s", srv.Addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	// ListenAndServe returns as soon as shutting down begins. Waiting for the
	// requests in flight to finish keeps the database open under them.
	<-drained
	return nil
}

// newMux routes every path the HTTP server answers.
func newMux(d storage.Database, subs fetch.Subscriptions) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", auth.HandleLogin(d))
	mux.HandleFunc("/logout", auth.HandleLogout(d))
	mux.HandleFunc("/fever/", api.FeverHandler(d))
	mux.HandleFunc("/greader/", api.GReaderHandler(d, subs))
	mux.HandleFunc("/version", handleVersion)
	mux.Handle("/cache", auth.WithAuth(cache.NewImageProxy(), d, *publicFolder, cache.DenyUnauthenticated, true))
	mux.Handle("/static/", http.FileServer(http.Dir(*publicFolder)))
	mux.Handle("/", auth.WithAuth(http.FileServer(http.Dir(*publicFolder)), d, *publicFolder, nil, false))
	return mux
}

func handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]any{
		"build_timestamp": buildTimestamp,
		"build_hash":      buildHash,
		// The newest migration the binary was built with, and the newest the
		// database had when it started. They differ only when the database is
		// ahead, on migrations the binary was allowed to run past.
		"schema_version":    schema.Latest(),
		"db_schema_version": dbSchemaVersion,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Warningf("Failed to encode response JSON: %s", err)
	}
}
