// Package devseed builds throwaway databases populated from an existing one,
// for exercising the server with many users holding realistic data.
//
// Nothing here is part of the server. Everything it copies is read from the
// source database at the time it runs, so the result reflects whatever that
// database holds and nothing about it is fixed in the code.
package devseed

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/jrupac/goliath/models"
	_ "github.com/lib/pq"
)

// identifier matches the database names this package will interpolate into
// SQL. Names cannot be passed as parameters, so anything else is refused
// rather than quoted.
var identifier = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func checkIdentifier(name string) error {
	if !identifier.MatchString(name) {
		return fmt.Errorf("database name %q must match %s", name, identifier)
	}
	return nil
}

// DSNFor returns the connection string for a database on the same cluster as
// dsn.
func DSNFor(dsn, database string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.Path = "/" + database
	return u.String(), nil
}

// CreateDatabase creates an empty database named name on the cluster dsn
// connects to, with the schema applied, and returns a connection string for
// it. The schema is the text of the current full schema, which creates and
// grants on a database of its own name; that name is replaced.
//
// dsn must connect as a user that can create databases and roles. The role the
// server connects as is created if missing, since the schema grants to it.
func CreateDatabase(ctx context.Context, dsn, name, schema string) (string, error) {
	if err := checkIdentifier(name); err != nil {
		return "", err
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return "", err
	}
	defer db.Close()

	// One connection throughout, since the schema switches the session's
	// database part way through.
	conn, err := db.Conn(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if _, err = conn.ExecContext(ctx, `CREATE USER IF NOT EXISTS goliath`); err != nil {
		return "", fmt.Errorf("creating role: %w", err)
	}
	var exists bool
	if err = conn.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM [SHOW DATABASES] WHERE database_name = $1)`, name).Scan(&exists); err != nil {
		return "", err
	}
	if exists {
		return "", fmt.Errorf("database %s already exists", name)
	}
	if _, err = conn.ExecContext(ctx, strings.ReplaceAll(schema, "Goliath", name)); err != nil {
		return "", fmt.Errorf("applying schema: %w", err)
	}
	return DSNFor(dsn, name)
}

// DropDatabase removes a database created by CreateDatabase.
func DropDatabase(ctx context.Context, dsn, name string) error {
	if err := checkIdentifier(name); err != nil {
		return err
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s CASCADE`, name))
	return err
}

// SourceFeed is a feed as found in the source database.
type SourceFeed struct {
	ID          int64
	Title       string
	Description string
	URL         string
	Link        string
}

// Options says what to copy and how.
type Options struct {
	// SourceDB names the database to copy from, on the same cluster as the
	// target. It is only read.
	SourceDB string

	// SourceUser is the username whose feeds are copied. Empty picks whoever
	// has the most feeds.
	SourceUser string

	// Share is the fraction of the source's feeds each user receives. The
	// shares are windows spaced evenly around the list, so with more users
	// than 1/Share each feed goes to several users.
	Share float64

	// Relocate, if set, gives the address and site link each copied feed is
	// stored under in place of its own, which is how a caller keeps fetches of
	// copied feeds away from the hosts that publish them. The retrieval cache
	// describes what was seen at the original address, so a relocated feed
	// does not get one.
	Relocate func(SourceFeed) (feedURL, link string)
}

// Copied records one feed copied to one user.
type Copied struct {
	User     models.User
	Source   SourceFeed
	FeedID   int64
	FolderID int64
	// Articles and Unread count what the copy holds.
	Articles int64
	Unread   int64
}

// Seed gives each user a share of the source user's feeds, with every
// article in them, filed in folders of the user's own.
//
// Each user gets a different folder layout: between one and four folders,
// some nested, with every so often a feed left unfiled, so that per-folder
// behaviour and the root folder are both exercised. Read and starred state is
// copied as it is, so each user starts with a realistic mix.
//
// The users must already exist, with the root folder every user has.
func Seed(ctx context.Context, db *sql.DB, users []models.User, opts Options) ([]Copied, error) {
	if err := checkIdentifier(opts.SourceDB); err != nil {
		return nil, err
	}
	if opts.Share <= 0 || opts.Share > 1 {
		return nil, fmt.Errorf("share %v must be in (0, 1]", opts.Share)
	}
	src := opts.SourceDB + ".public."

	sourceUser, err := findSourceUser(ctx, db, src, opts.SourceUser)
	if err != nil {
		return nil, err
	}
	feeds, err := listSourceFeeds(ctx, db, src, sourceUser)
	if err != nil {
		return nil, err
	}
	if len(feeds) == 0 {
		return nil, fmt.Errorf("source user has no feeds")
	}

	width := max(1, int(float64(len(feeds))*opts.Share+0.5))
	var copied []Copied
	for i, u := range users {
		folders, err := makeFolders(ctx, db, u, i)
		if err != nil {
			return nil, fmt.Errorf("making folders for %s: %w", u.Username, err)
		}
		start := i * len(feeds) / len(users)
		for j := range width {
			f := feeds[(start+j)%len(feeds)]
			c, err := copyFeed(ctx, db, src, sourceUser, u, f, folders[j%len(folders)], opts.Relocate)
			if err != nil {
				return nil, fmt.Errorf("copying feed %d to %s: %w", f.ID, u.Username, err)
			}
			copied = append(copied, c)
		}
	}
	return copied, nil
}

func findSourceUser(ctx context.Context, db *sql.DB, src, username string) (models.UserId, error) {
	var id models.UserId
	var err error
	if username != "" {
		err = db.QueryRowContext(ctx,
			`SELECT id FROM `+src+`usertable WHERE username = $1`, username).Scan(&id)
	} else {
		err = db.QueryRowContext(ctx, `
			SELECT userid FROM `+src+`feed WHERE deleted IS NULL
			GROUP BY userid ORDER BY count(*) DESC LIMIT 1`).Scan(&id)
	}
	if err != nil {
		return "", fmt.Errorf("finding source user: %w", err)
	}
	return id, nil
}

func listSourceFeeds(ctx context.Context, db *sql.DB, src string, user models.UserId) ([]SourceFeed, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, coalesce(title, ''), coalesce(description, ''), coalesce(url, ''), coalesce(link, '')
		FROM `+src+`feed WHERE userid = $1 AND deleted IS NULL`, user)
	if err != nil {
		return nil, fmt.Errorf("listing source feeds: %w", err)
	}
	defer rows.Close()

	var feeds []SourceFeed
	for rows.Next() {
		var f SourceFeed
		if err = rows.Scan(&f.ID, &f.Title, &f.Description, &f.URL, &f.Link); err != nil {
			return nil, err
		}
		feeds = append(feeds, f)
	}
	sort.Slice(feeds, func(a, b int) bool { return feeds[a].ID < feeds[b].ID })
	return feeds, rows.Err()
}

// makeFolders creates the i-th user's folders and returns the IDs feeds are
// dealt into in turn, the root among them.
func makeFolders(ctx context.Context, db *sql.DB, u models.User, i int) ([]int64, error) {
	var root int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM Folder WHERE userid = $1 AND name = $2`,
		u.UserId, models.RootFolder).Scan(&root); err != nil {
		return nil, fmt.Errorf("finding root folder: %w", err)
	}

	n := 1 + i%4
	ids := make([]int64, 0, n+1)
	for k := range n {
		parent := root
		// Every third user has their last folder nested inside their first.
		if k == n-1 && k > 0 && i%3 == 2 {
			parent = ids[0]
		}
		var id int64
		if err := db.QueryRowContext(ctx, `
			INSERT INTO Folder (userid, name) VALUES ($1, $2)
			ON CONFLICT (userid, name) DO UPDATE SET name = excluded.name
			RETURNING id`, u.UserId, fmt.Sprintf("Folder %d", k+1)).Scan(&id); err != nil {
			return nil, err
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO FolderChildren (userid, parent, child) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, u.UserId, parent, id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return append(ids, root), nil
}

func copyFeed(ctx context.Context, db *sql.DB, src string, sourceUser models.UserId, u models.User,
	f SourceFeed, folder int64, relocate func(SourceFeed) (string, string)) (Copied, error) {
	c := Copied{User: u, Source: f, FolderID: folder}

	stored := models.Feed{Title: f.Title, Description: f.Description, URL: f.URL, Link: f.Link}
	if relocate != nil {
		stored.URL, stored.Link = relocate(f)
	}

	err := db.QueryRowContext(ctx, `
		INSERT INTO Feed (userid, folder, hash, title, description, url, link,
		                  mime, favicon, latest, estimated_refresh_interval, titleoverridden)
		SELECT $1, $2, $3, $4, $5, $6, $7, mime, favicon, latest, estimated_refresh_interval, titleoverridden
		FROM `+src+`feed WHERE id = $8
		RETURNING id`,
		u.UserId, folder, stored.Hash(), stored.Title, stored.Description, stored.URL, stored.Link, f.ID,
	).Scan(&c.FeedID)
	if err != nil {
		return c, fmt.Errorf("inserting feed: %w", err)
	}

	if _, err = db.ExecContext(ctx, `
		INSERT INTO Article (userid, feed, hash, title, summary, content, parsed, link,
		                     read, saved, date, retrieved, readat)
		SELECT $1, $2, hash, title, summary, content, parsed, link, read, saved, date, retrieved, readat
		FROM `+src+`article WHERE userid = $3 AND feed = $4`,
		u.UserId, c.FeedID, sourceUser, f.ID); err != nil {
		return c, fmt.Errorf("copying articles: %w", err)
	}

	if relocate == nil {
		if _, err = db.ExecContext(ctx, `
			INSERT INTO RetrievalCache (userid, feedid, cache)
			SELECT $1, $2, cache FROM `+src+`retrievalcache WHERE userid = $3 AND feedid = $4`,
			u.UserId, c.FeedID, sourceUser, f.ID); err != nil {
			return c, fmt.Errorf("copying retrieval cache: %w", err)
		}
	}

	err = db.QueryRowContext(ctx, `
		SELECT count(*), count(*) FILTER (WHERE NOT read) FROM Article WHERE userid = $1 AND feed = $2`,
		u.UserId, c.FeedID).Scan(&c.Articles, &c.Unread)
	return c, err
}
