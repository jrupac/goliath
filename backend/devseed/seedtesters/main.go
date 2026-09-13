// Command seedtesters builds a throwaway database of test users, each with a
// share of an existing user's feeds and articles, and serves those feeds from a
// local address until interrupted.
//
// Everything copied is read from the source database when this runs. Each
// copied feed is moved to the local feed server, which serves it with no items
// until some are published to it, so a server pointed at the database fetches
// nothing from the real publishers. Run from the backend directory:
//
//	go run ./devseed/seedtesters -db testers -source <database>
//
// The database is dropped on exit unless -keep is given; -reuse serves the
// feeds of one built earlier instead of building a new one.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/jrupac/goliath/devseed"
	"github.com/jrupac/goliath/feedtest"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
)

var (
	crdbDSN = flag.String("crdb", "postgresql://root@localhost:26257/defaultdb?sslmode=disable",
		"Connection string for the cluster, as a user that can create databases.")
	target     = flag.String("db", "", "Name of the database to create, or with -reuse, to serve.")
	source     = flag.String("source", "", "Name of the database to copy feeds and articles from. Only read.")
	sourceUser = flag.String("source-user", "", "Username to copy from; defaults to whoever has the most feeds.")
	users      = flag.Int("users", 10, "Number of users to create.")
	prefix     = flag.String("prefix", "tester", "Usernames are this followed by a two-digit number.")
	share      = flag.Float64("share", 0.4, "Fraction of the source's feeds each user receives.")
	schemaPath = flag.String("schema", "schema/latest.sql", "Path to the full schema.")
	feedsAddr  = flag.String("feeds", "127.0.0.1:9980", "Address to serve the copied feeds from.")
	keep       = flag.Bool("keep", false, "Keep the database on exit.")
	reuse      = flag.Bool("reuse", false, "Serve the feeds of an existing database built by this command.")
)

// password is what each test user's password is, so that it can be printed
// again for a database being reused.
func password(username string) string { return username + "-password" }

func main() {
	flag.Parse()
	if *target == "" || (*source == "" && !*reuse) {
		log.Fatal("-db is required, and -source unless -reuse is given")
	}
	ctx := context.Background()

	feeds, err := feedtest.Start(*feedsAddr)
	if err != nil {
		log.Fatalf("serving feeds on %s: %s", *feedsAddr, err)
	}
	defer feeds.Close()

	var dsn string
	if *reuse {
		if dsn, err = devseed.DSNFor(*crdbDSN, *target); err != nil {
			log.Fatal(err)
		}
	} else {
		schema, err := os.ReadFile(*schemaPath)
		if err != nil {
			log.Fatalf("reading schema: %s", err)
		}
		if dsn, err = devseed.CreateDatabase(ctx, *crdbDSN, *target, string(schema)); err != nil {
			log.Fatalf("creating database: %s", err)
		}
	}
	if !*keep {
		defer func() {
			if err := devseed.DropDatabase(context.Background(), *crdbDSN, *target); err != nil {
				log.Printf("dropping %s: %s", *target, err)
			} else {
				fmt.Printf("Dropped %s.\n", *target)
			}
		}()
	}

	raw, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("opening %s: %s", *target, err)
	}
	defer raw.Close()

	if *reuse {
		err = serveExisting(ctx, raw, feeds)
	} else {
		err = build(ctx, dsn, raw, feeds)
	}
	if err != nil {
		log.Print(err)
		return
	}

	if err = printUsers(ctx, raw); err != nil {
		log.Print(err)
		return
	}
	appDSN, _ := devseed.DSNFor("postgresql://goliath@localhost:26257/?sslmode=disable", *target)
	fmt.Printf(`
Serving feeds on http://%[1]s. Run a server on this machine (not in a
container, which cannot reach this address) with:

  -dbPath='%[2]s' -feedAllowedAddresses=%[1]s

Publish new items to a feed with:

  curl -X POST 'http://%[1]s/_publish?path=<path>&n=3'

and list the feeds, their items and how often each was fetched at
http://%[1]s/_feeds. Interrupt to stop%[3]s.
`, feeds.Addr(), appDSN, map[bool]string{true: "", false: " and drop " + *target}[*keep])

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
}

// build creates the test users and copies their feeds, moving each to the
// local feed server.
func build(ctx context.Context, dsn string, raw *sql.DB, feeds *feedtest.Server) error {
	d, err := storage.Open(dsn)
	if err != nil {
		return fmt.Errorf("opening %s: %w", *target, err)
	}
	defer d.Close()

	var created []models.User
	for i := range *users {
		name := fmt.Sprintf("%s%02d", *prefix, i+1)
		u, err := models.NewUser(name, models.Secret(password(name)))
		if err != nil {
			return fmt.Errorf("user %s: %w", name, err)
		}
		if u, err = d.InsertUser(u); err != nil {
			return fmt.Errorf("adding %s: %w", name, err)
		}
		created = append(created, u)
	}

	_, err = devseed.Seed(ctx, raw, created, devseed.Options{
		SourceDB:   *source,
		SourceUser: *sourceUser,
		Share:      *share,
		Relocate: func(f devseed.SourceFeed) (string, string) {
			feedURL := feeds.Add(fmt.Sprintf("/seeded/%d", f.ID), f.Title, 0)
			return feedURL, feedURL + "/site"
		},
	})
	if err != nil {
		return fmt.Errorf("seeding: %w", err)
	}
	return nil
}

// serveExisting registers every feed an earlier run moved to the feed server's
// address, so that they are served again.
func serveExisting(ctx context.Context, raw *sql.DB, feeds *feedtest.Server) error {
	base := feeds.URL("/")
	rows, err := raw.QueryContext(ctx,
		`SELECT DISTINCT ON (url) url, title FROM Feed WHERE url LIKE $1 || '%'`, base)
	if err != nil {
		return fmt.Errorf("listing feeds: %w", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var url, title string
		if err = rows.Scan(&url, &title); err != nil {
			return err
		}
		feeds.Add("/"+strings.TrimPrefix(url, base), title, 0)
		n++
	}
	if n == 0 {
		return fmt.Errorf("no feeds in %s are at %s; was it built with this -feeds address?", *target, base)
	}
	return rows.Err()
}

func printUsers(ctx context.Context, raw *sql.DB) error {
	rows, err := raw.QueryContext(ctx, `
		SELECT u.username, u.key,
		       (SELECT count(*) FROM Feed f WHERE f.userid = u.id AND f.deleted IS NULL),
		       (SELECT count(*) FROM Article a WHERE a.userid = u.id),
		       (SELECT count(*) FROM Article a WHERE a.userid = u.id AND NOT a.read)
		FROM UserTable u WHERE u.deleted IS NULL ORDER BY u.username`)
	if err != nil {
		return fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	fmt.Printf("Test users in %s:\n\n", *target)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "USERNAME\tPASSWORD\tFEVER KEY\tFEEDS\tARTICLES\tUNREAD")
	for rows.Next() {
		var name, key string
		var feedCount, articles, unread int64
		if err = rows.Scan(&name, &key, &feedCount, &articles, &unread); err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%d\n", name, password(name), key, feedCount, articles, unread)
	}
	w.Flush()
	return rows.Err()
}
