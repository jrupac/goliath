package storage

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/opml"
	"github.com/lib/pq"
)

// MaxFetchedRows is the most rows a single content read returns. A handler
// taking a page size from a client must clamp to it rather than pass the
// client's number through, since nothing on the wire bounds what is asked for.
const MaxFetchedRows = 10000

const (
	dialect            = "postgres"
	slowOpLogThreshold = 50 * time.Millisecond
	retryBackoff       = 1 * time.Second
	maxOperationTime   = 30 * time.Second
)

// Crdb is a wrapper type around a database connection.
type Crdb struct {
	db *sql.DB
}

// Open opens a connection to the given database path and tests connectivity.
func (crdb *Crdb) Open(dbPath string) error {
	var d *sql.DB
	var err error

	i := 0
	for i < *openRetries {
		d, err = sql.Open(dialect, dbPath)
		if err == nil {
			break
		}
		log.Warningf("sql.Open got error: %v, waiting 1s and retrying...", err)
		time.Sleep(retryBackoff)
		i += 1
	}

	if d == nil || err != nil {
		log.Errorf("could not open DB after %d retries, failing.", *openRetries)
		return err
	} else {
		log.Infof("Successfully connected to DB!")
	}

	i = 0
	for i < *pingRetries {
		err = d.Ping()
		if err == nil {
			break
		}

		log.Warningf("sql.Ping got error: %v, waiting 1s and retrying...", err)
		time.Sleep(retryBackoff)
		i += 1
	}

	if err != nil {
		log.Errorf("could not ping DB after %d retries, failing.", *pingRetries)
		return err
	} else {
		log.Infof("Successfully pinged DB!")
	}

	crdb.db = d
	return nil
}

// Close closes the database connection.
func (crdb *Crdb) Close() error {
	return crdb.db.Close()
}

/*******************************************************************************
 * User management
 ******************************************************************************/

// InsertUser inserts the given user into the database.
func (crdb *Crdb) InsertUser(u models.User) error {
	defer logElapsedTime(time.Now(), "InsertUser")

	query := `INSERT INTO UserTable (id, username, key) VALUES($1, $2, $3)`
	_, err := crdb.db.Exec(query, u.UserId, u.Username, u.Key)
	if err != nil {
		return err
	}

	// Also add an empty mute_word column for the new user
	query = `
		INSERT INTO UserPrefs (userid, mute_words)
		VALUES($1, ARRAY[]::STRING[])
	`
	_, err = crdb.db.Exec(query, u.UserId)

	return err
}

// GetAllUsers returns a list of all models.User objects.
func (crdb *Crdb) GetAllUsers() ([]models.User, error) {
	defer logElapsedTime(time.Now(), "GetAllUsers")

	var users []models.User

	query := `SELECT id, username, key FROM UserTable`
	rows, err := crdb.db.Query(query)
	defer closeSilent(rows)

	if err != nil {
		return users, err
	}

	for rows.Next() {
		u := models.User{}
		if err = rows.Scan(&u.UserId, &u.Username, &u.Key); err != nil {
			return users, err
		}
		users = append(users, u)
	}

	return users, err
}

// GetUserByKey returns a user identified by the given key.
func (crdb *Crdb) GetUserByKey(key models.Secret) (models.User, error) {
	defer logElapsedTime(time.Now(), "GetUserByKey")

	var u models.User

	query := `SELECT id, username, key, hashpass FROM UserTable WHERE key = $1`
	err := crdb.db.QueryRow(query, key).Scan(&u.UserId, &u.Username, &u.Key, &u.HashPass)

	if !u.Valid() {
		return models.User{}, errors.New("could not find user")
	}

	return u, err
}

// GetUserByUsername returns a user identified by the given username.
func (crdb *Crdb) GetUserByUsername(username string) (models.User, error) {
	defer logElapsedTime(time.Now(), "GetUserByUsername")

	var u models.User

	query := `SELECT id, username, key, hashpass FROM UserTable WHERE username = $1`
	err := crdb.db.QueryRow(query, username).Scan(&u.UserId, &u.Username, &u.Key, &u.HashPass)

	if !u.Valid() {
		return models.User{}, errors.New("could not find user")
	}
	return u, err
}

// UpdateUserCredentials replaces the stored password hash and derived key for a
// user.
//
// Both move together on purpose. The key is derived from the password, so
// leaving it behind would mean the old password still opened the Fever API and
// the web cookie, and the change would not be the revocation it looks like.
func (crdb *Crdb) UpdateUserCredentials(u models.User, hashPass models.Secret, key models.Secret) error {
	defer logElapsedTime(time.Now(), "UpdateUserCredentials")

	query := `UPDATE UserTable SET hashpass = $1, key = $2 WHERE id = $3`
	res, err := crdb.db.Exec(query, hashPass, key, u.UserId)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("could not find user")
	}
	return nil
}

/*******************************************************************************
 * Sessions
 ******************************************************************************/

// CreateSession establishes a new session for the given user and returns its
// bearer token. The token is returned here and nowhere else: only its digest is
// stored, so it cannot be recovered afterwards.
func (crdb *Crdb) CreateSession(u models.User, scheme models.AuthScheme, userAgent string) (models.Secret, error) {
	defer logElapsedTime(time.Now(), "CreateSession")

	token, err := newSessionToken()
	if err != nil {
		return "", err
	}

	query := `
		INSERT INTO Session (tokenhash, userid, scheme, useragent)
		VALUES ($1, $2, $3, $4)`
	if _, err = crdb.db.Exec(query, hashSessionToken(token), u.UserId, string(scheme), userAgent); err != nil {
		return "", err
	}

	return token, nil
}

// LookupSession resolves a bearer token to its session and the user it belongs
// to. Tokens that do not exist and sessions that have gone idle past the expiry
// window are both reported as an error, so a caller cannot distinguish them.
//
// This also advances the session's last-seen time, which is what makes expiry
// slide. That write is rate-limited: authenticating is a read on the hot path
// and every request would otherwise turn it into a write.
func (crdb *Crdb) LookupSession(token models.Secret) (models.User, models.Session, error) {
	defer logElapsedTime(time.Now(), "LookupSession")

	var (
		s models.Session
		u models.User
	)

	hash := hashSessionToken(token)
	var scheme string
	query := `
		SELECT s.id, s.userid, s.scheme, s.created, s.lastseen, s.useragent,
		       u.username, u.key, u.hashpass
		FROM Session s JOIN UserTable u ON u.id = s.userid
		WHERE s.tokenhash = $1 AND s.lastseen > $2`
	err := crdb.db.QueryRow(query, hash, SessionExpiryCutoff()).Scan(
		&s.SessionId, &s.UserId, &scheme, &s.Created, &s.LastSeen, &s.UserAgent,
		&u.Username, &u.Key, &u.HashPass)
	if err != nil {
		return models.User{}, models.Session{}, errors.New("no such session")
	}
	u.UserId = s.UserId

	// Parsed rather than converted, so that a scheme this build does not know
	// how to issue does not authenticate anyone.
	if s.Scheme, err = models.ParseAuthScheme(scheme); err != nil {
		log.Warningf("Session %s carries an unusable scheme: %s", s.SessionId, err)
		return models.User{}, models.Session{}, errors.New("no such session")
	}

	if !s.Valid() || !u.Valid() {
		return models.User{}, models.Session{}, errors.New("no such session")
	}

	if time.Since(s.LastSeen) > *sessionTouchInterval {
		if err = crdb.touchSession(hash); err != nil {
			// The session is still good; only the expiry slide was lost, and
			// the next request will try again.
			log.Warningf("Failed to record use of session %s: %s", s.SessionId, err)
		}
	}

	return u, s, nil
}

func (crdb *Crdb) touchSession(hash []byte) error {
	query := `UPDATE Session SET lastseen = now() WHERE tokenhash = $1`
	_, err := crdb.db.Exec(query, hash)
	return err
}

// GetSessionsForUser returns all unexpired sessions belonging to the given
// user, most recently used first.
func (crdb *Crdb) GetSessionsForUser(u models.User) ([]models.Session, error) {
	defer logElapsedTime(time.Now(), "GetSessionsForUser")

	var sessions []models.Session

	query := `
		SELECT id, userid, scheme, created, lastseen, useragent
		FROM Session
		WHERE userid = $1 AND lastseen > $2
		ORDER BY lastseen DESC`
	rows, err := crdb.db.Query(query, u.UserId, SessionExpiryCutoff())
	defer closeSilent(rows)

	if err != nil {
		return sessions, err
	}

	for rows.Next() {
		s := models.Session{}
		var scheme string
		if err = rows.Scan(&s.SessionId, &s.UserId, &scheme, &s.Created, &s.LastSeen, &s.UserAgent); err != nil {
			return sessions, err
		}
		// A listing exists to be acted on, so an unrecognized scheme is shown
		// rather than hidden: it is still a session someone may want revoked.
		if s.Scheme, err = models.ParseAuthScheme(scheme); err != nil {
			log.Warningf("Session %s carries an unusable scheme: %s", s.SessionId, err)
			s.Scheme = ""
		}
		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// DeleteSessionForUser revokes a single session. The user is part of the
// predicate so that knowing a session ID is not by itself enough to revoke it.
func (crdb *Crdb) DeleteSessionForUser(u models.User, id models.SessionId) (int64, error) {
	defer logElapsedTime(time.Now(), "DeleteSessionForUser")

	query := `DELETE FROM Session WHERE id = $1 AND userid = $2`
	res, err := crdb.db.Exec(query, id, u.UserId)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteSessionsForUser revokes every session belonging to the given user.
func (crdb *Crdb) DeleteSessionsForUser(u models.User) (int64, error) {
	defer logElapsedTime(time.Now(), "DeleteSessionsForUser")

	query := `DELETE FROM Session WHERE userid = $1`
	res, err := crdb.db.Exec(query, u.UserId)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteExpiredSessions removes sessions last used before the given time.
// Expiry is already enforced on lookup, so this only reclaims rows.
func (crdb *Crdb) DeleteExpiredSessions(before time.Time) (int64, error) {
	defer logElapsedTime(time.Now(), "DeleteExpiredSessions")

	query := `DELETE FROM Session WHERE lastseen <= $1`
	res, err := crdb.db.Exec(query, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

/*******************************************************************************
 * User preferences
 ******************************************************************************/

// GetMuteWordsForUser returns a list of mute words for the given user.
func (crdb *Crdb) GetMuteWordsForUser(u models.User) ([]string, error) {
	defer logElapsedTime(time.Now(), "GetMuteWordsForUser")

	var words pq.StringArray

	query := `SELECT mute_words FROM UserPrefs WHERE userid = $1`
	err := crdb.db.QueryRow(query, u.UserId).Scan(&words)

	return words, err
}

// UpdateMuteWordsForUser updates the mute words for a given user with the
// provided mute words, maintaining a sorted order.
func (crdb *Crdb) UpdateMuteWordsForUser(u models.User, words []string) error {
	defer logElapsedTime(time.Now(), "UpdateMuteWordsForUser")

	query := `
		INSERT INTO UserPrefs (userid, mute_words)
		VALUES ($1, $2)
		ON CONFLICT (userid) DO UPDATE
		SET mute_words = (
			SELECT array_agg(word ORDER BY word) 
			FROM (
				SELECT DISTINCT unnest(array_cat(UserPrefs.mute_words, $2::STRING[])) AS word
			) AS unique_words
		)
	`
	_, err := crdb.db.Exec(query, u.UserId, pq.Array(words))

	return err
}

// DeleteMuteWordsForUser deletes the mute words for a given user, maintaining
// the sorted order.
func (crdb *Crdb) DeleteMuteWordsForUser(u models.User, words []string) error {
	defer logElapsedTime(time.Now(), "DeleteMuteWordsForUser")

	query := `
		UPDATE UserPrefs
		SET mute_words = (
			SELECT array_agg(word ORDER BY word)
			FROM (
				SELECT DISTINCT unnest(mute_words) AS word
				EXCEPT
				SELECT unnest($2::STRING[])
			) AS filtered_words
		)
		WHERE userid = $1
	`
	_, err := crdb.db.Exec(query, u.UserId, pq.Array(words))

	return err
}

// GetUnmuteFeedsForUser returns a list of unmuted feed IDs for the given user.
func (crdb *Crdb) GetUnmuteFeedsForUser(u models.User) ([]int64, error) {
	defer logElapsedTime(time.Now(), "GetUnmuteFeedsForUser")

	var feedIds []int64

	query := `SELECT feedid FROM UserUnmuteFeeds WHERE userid = $1`
	rows, err := crdb.db.Query(query, u.UserId)
	defer closeSilent(rows)
	if err != nil {
		return feedIds, err
	}

	for rows.Next() {
		var feedId int64
		if err = rows.Scan(&feedId); err != nil {
			return feedIds, err
		}
		feedIds = append(feedIds, feedId)
	}
	return feedIds, err
}

// UpdateUnmuteFeedsForUser inserts unmute feed IDs for the given user.
func (crdb *Crdb) UpdateUnmuteFeedsForUser(u models.User, feedIds []int64) error {
	defer logElapsedTime(time.Now(), "UpdateUnmuteFeedsForUser")

	for _, feedId := range feedIds {
		query := `INSERT INTO UserUnmuteFeeds(userid, feedid) VALUES ($1, $2) ON CONFLICT DO NOTHING`
		_, err := crdb.db.Exec(query, u.UserId, feedId)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteUnmuteFeedsForUser deletes unmute feed IDs for a given user.
func (crdb *Crdb) DeleteUnmuteFeedsForUser(u models.User, feedIds []int64) error {
	defer logElapsedTime(time.Now(), "DeleteUnmuteFeedsForUser")

	if len(feedIds) == 0 {
		return nil
	}

	query := `DELETE FROM UserUnmuteFeeds WHERE userid = $1 AND feedid = ANY($2)`

	_, err := crdb.db.Exec(query, u.UserId, pq.Array(feedIds))

	return err

}

// GetFeedMuteRegexesForUser returns all feed mute regexes for a given user.
func (crdb *Crdb) GetFeedMuteRegexesForUser(u models.User) (map[int64][]string, error) {
	defer logElapsedTime(time.Now(), "GetFeedMuteRegexesForUser")

	ret := make(map[int64][]string)

	query := `SELECT feedid, regex FROM UserFeedMuteRegexes WHERE userid = $1`
	rows, err := crdb.db.Query(query, u.UserId)
	defer closeSilent(rows)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var feedId int64
		var regex string
		if err = rows.Scan(&feedId, &regex); err != nil {
			return nil, err
		}
		ret[feedId] = append(ret[feedId], regex)
	}

	return ret, err
}

// GetMuteRegexesForFeedForUser returns the mute regexes for a specific user and feed.
func (crdb *Crdb) GetMuteRegexesForFeedForUser(u models.User, feedId int64) ([]string, error) {
	defer logElapsedTime(time.Now(), "GetMuteRegexesForFeedForUser")

	var regexes []string

	query := `SELECT regex FROM UserFeedMuteRegexes WHERE userid = $1 AND feedid = $2`
	rows, err := crdb.db.Query(query, u.UserId, feedId)
	defer closeSilent(rows)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var regex string
		if err = rows.Scan(&regex); err != nil {
			return nil, err
		}
		regexes = append(regexes, regex)
	}

	return regexes, err
}

// AddMuteRegexForFeedForUser adds a feed mute regex for a given user and feed.
func (crdb *Crdb) AddMuteRegexForFeedForUser(u models.User, feedId int64, regex string) error {
	defer logElapsedTime(time.Now(), "AddMuteRegexForFeedForUser")

	query := `INSERT INTO UserFeedMuteRegexes (userid, feedid, regex) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := crdb.db.Exec(query, u.UserId, feedId, regex)

	return err
}

// DeleteMuteRegexForFeedForUser deletes a specific feed mute regex for a given user and feed.
func (crdb *Crdb) DeleteMuteRegexForFeedForUser(u models.User, feedId int64, regex string) error {
	defer logElapsedTime(time.Now(), "DeleteMuteRegexForFeedForUser")

	query := `DELETE FROM UserFeedMuteRegexes WHERE userid = $1 AND feedid = $2 AND regex = $3`
	_, err := crdb.db.Exec(query, u.UserId, feedId, regex)

	return err
}

/*******************************************************************************
 * Retrieval cache
 ******************************************************************************/

// GetActiveFeedKeys retrieves the keys of all feeds that still have a row,
// including those unsubscribed from and awaiting garbage collection. A feed
// can be restored until then, and it should come back remembering what it had
// already fetched.
func (crdb *Crdb) GetActiveFeedKeys() (map[UserFeedKey]bool, error) {
	defer logElapsedTime(time.Now(), "GetActiveFeedKeys")
	return crdb.feedKeys(`SELECT userid, id FROM Feed`)
}

// GetLiveFeedKeys retrieves the keys of every feed any user is subscribed to.
func (crdb *Crdb) GetLiveFeedKeys() (map[UserFeedKey]bool, error) {
	defer logElapsedTime(time.Now(), "GetLiveFeedKeys")
	return crdb.feedKeys(`SELECT userid, id FROM Feed WHERE deleted IS NULL`)
}

func (crdb *Crdb) feedKeys(query string) (map[UserFeedKey]bool, error) {
	rows, err := crdb.db.Query(query)
	defer closeSilent(rows)

	if err != nil {
		return nil, err
	}

	ret := map[UserFeedKey]bool{}

	for rows.Next() {
		var userID models.UserId
		var feedID int64
		if err = rows.Scan(&userID, &feedID); err != nil {
			return nil, err
		}
		ret[UserFeedKey{UserID: userID, FeedID: feedID}] = true
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ret, nil
}

// GetAllRetrievalCaches retrieves the cache for all feeds.
func (crdb *Crdb) GetAllRetrievalCaches() (map[UserFeedKey]string, error) {
	defer logElapsedTime(time.Now(), "GetAllRetrievalCaches")

	query := `SELECT userid, feedid, cache FROM RetrievalCache`
	rows, err := crdb.db.Query(query)
	defer closeSilent(rows)

	if err != nil {
		return nil, err
	}

	ret := map[UserFeedKey]string{}

	for rows.Next() {
		var userID models.UserId
		var feedID int64
		var cache string
		if err = rows.Scan(&userID, &feedID, &cache); err != nil {
			return nil, err
		}
		ret[UserFeedKey{UserID: userID, FeedID: feedID}] = cache
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ret, nil
}

// PersistAllRetrievalCaches writes the retrieval caches for all feeds.
func (crdb *Crdb) PersistAllRetrievalCaches(entries map[UserFeedKey][]byte) error {
	defer logElapsedTime(time.Now(), "PersistAllRetrievalCaches")

	ctx, cancel := context.WithTimeout(context.Background(), maxOperationTime)
	defer cancel()
	tx, err := crdb.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSilent(tx)

	query := `UPSERT INTO RetrievalCache (userid, feedid, cache) VALUES ($1, $2, $3)`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}

	for key, cache := range entries {
		// TODO: Change to BYTE type for this column instead of string encoding
		encodedCache := base64.StdEncoding.EncodeToString(cache)
		_, err = stmt.ExecContext(ctx, key.UserID, key.FeedID, encodedCache)
		if err != nil {
			return fmt.Errorf("failed to execute statement: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

/*******************************************************************************
 * Content insertion
 ******************************************************************************/

// ErrFeedGone reports a write into a feed that is not live: unsubscribed from,
// or removed outright. A fetch that was already running when that happened is
// the ordinary way to get it, and such a fetch should stop rather than retry.
var ErrFeedGone = errors.New("feed is not subscribed")

// InsertArticleForUser inserts the given article object into the database,
// reporting ErrFeedGone if its feed is not live.
//
// The article's folder is read from its feed inside the statement rather than
// taken from the article. A feed's folder is part of the key its articles are
// stored under, so a fetch that read the feed before it was moved would
// otherwise write rows naming the old folder, which the foreign key refuses.
func (crdb *Crdb) InsertArticleForUser(u models.User, a models.Article) error {
	defer logElapsedTime(time.Now(), "InsertArticleForUser")

	query := `
		WITH live AS (
			SELECT folder FROM Feed WHERE userid = $1 AND id = $2 AND deleted IS NULL
		), inserted AS (
			INSERT INTO Article (userid, folder, feed, hash, title, summary, content, parsed, link, read, saved, date, retrieved)
			SELECT $1::UUID, folder, $2::INT8, $3::STRING, $4::STRING, $5::STRING, $6::STRING, $7::STRING,
			       $8::STRING, $9::BOOL, $10::BOOL, $11::TIMESTAMPTZ, $12::TIMESTAMPTZ
			FROM live
			ON CONFLICT (userid, feed, hash) DO NOTHING
			RETURNING id
		)
		SELECT (SELECT count(*) FROM live), (SELECT count(*) FROM inserted)
	`
	var live, inserted int
	err := crdb.db.QueryRow(query,
		u.UserId, a.FeedID, a.Hash(), a.Title, a.Summary, a.Content, a.Parsed, a.Link, a.Read, a.Saved, a.Date, a.Retrieved,
	).Scan(&live, &inserted)
	if err != nil {
		return fmt.Errorf("failed to insert article: %w", err)
	}

	if live == 0 {
		return ErrFeedGone
	}
	if inserted == 0 {
		log.V(2).Infof("Duplicate article entry, skipping (hash): %s", a.Hash())
	}
	return nil
}

// InsertFaviconForUser inserts the given favicon and associated metadata into
// the database.
func (crdb *Crdb) InsertFaviconForUser(u models.User, feedId int64, mime string, img []byte) error {
	defer logElapsedTime(time.Now(), "InsertFaviconForUser")

	// TODO: Consider wrapping this into a Favicon model type.
	// Convert to a base64 encoded string before inserting
	h := base64.StdEncoding.EncodeToString(img)

	query := `
		UPDATE Feed
		SET favicon = $1, mime = $2
		WHERE userid = $3 AND id = $4 AND deleted IS NULL
	`
	result, err := crdb.db.Exec(query, h, mime, u.UserId, feedId)
	if err != nil {
		return fmt.Errorf("failed to update favicon: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrFeedGone
	}

	return nil
}

// InsertFeedForUser inserts a new feed into the database. If `folderId` is 0,
// the feed is assumed to be a top-level entry. Otherwise, the feed will be
// nested under the folder with that ID. If the root folder does not exist,
// returns -1 as the feed ID.
//
// A feed that collides with one the user unsubscribed from is that feed
// restored, rather than a live ID for a row every read leaves out.
func (crdb *Crdb) InsertFeedForUser(u models.User, f models.Feed, folderId int64) (int64, error) {
	defer logElapsedTime(time.Now(), "InsertFeedForUser")

	var feedID int64

	// If the feed is assumed to be a top-level entry, determine the ID of the
	// root folder that it actually is under.
	if folderId == 0 {
		root, err := crdb.GetRootFolderForUser(u)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return -1, fmt.Errorf("root folder (%s) not found for user %s", models.RootFolder, u.UserId)
			}
			return -1, fmt.Errorf("failed to get root folder ID: %w", err)
		}
		folderId = root.ID
	}

	query := `
		INSERT INTO Feed(userid, folder, hash, title, description, url, link)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT(userid, hash) DO UPDATE SET
			folder = excluded.folder,
			title = excluded.title,
			description = excluded.description,
			url = excluded.url,
			link = excluded.link,
			deleted = NULL
		RETURNING id
	`
	err := crdb.db.QueryRow(query, u.UserId, folderId, f.Hash(), f.Title, f.Description, f.URL, f.Link).Scan(&feedID)
	return feedID, err
}

// InsertFolderForUser inserts a new folder into the database, nested under the
// folder with ID `parentId`. A `parentId` of 0 files it under the root folder,
// or, for the root folder itself, under nothing. On error, -1 is returned for
// the folder ID.
func (crdb *Crdb) InsertFolderForUser(u models.User, f models.Folder, parentId int64) (int64, error) {
	defer logElapsedTime(time.Now(), "InsertFolderForUser")

	errFolderId := int64(-1)

	// Checked here rather than at each caller, so that no path to creating a
	// folder can miss it: feeds arrive from OPML files, the admin service and
	// clients, and they all land on this method.
	if err := models.ValidateFolderName(f.Name); err != nil {
		return errFolderId, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), maxOperationTime)
	defer cancel()
	tx, err := crdb.db.BeginTx(ctx, nil)
	if err != nil {
		return errFolderId, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSilent(tx)

	var folderID int64
	query := `
		INSERT INTO Folder(userid, name) VALUES($1, $2)
		ON CONFLICT(userid, name) DO UPDATE SET name = excluded.name 
		RETURNING id
	`
	err = tx.QueryRowContext(ctx, query, u.UserId, f.Name).Scan(&folderID)
	if err != nil {
		return errFolderId, fmt.Errorf("failed to insert folder: %w", err)
	}

	switch {
	case parentId != 0:
		query = `
			INSERT INTO FolderChildren(userid, parent, child) 
			VALUES($1, $2, $3)
			ON CONFLICT (userid, parent, child) DO NOTHING
		`
		_, err = tx.ExecContext(ctx, query, u.UserId, parentId, folderID)
		if err != nil {
			return errFolderId, fmt.Errorf("failed to insert into FolderChildren: %w", err)
		}
	case f.Name != models.RootFolder:
		// Filed under the root, which is where a folder given no parent belongs.
		// Unlinked, it would still hold feeds and still be listed to clients,
		// but anything walking the hierarchy from the root -- OPML export --
		// would pass over it and every feed in it. A folder the insert merged
		// into keeps whatever parent it already has.
		query = `
			INSERT INTO FolderChildren(userid, parent, child)
			SELECT userid, id, $2 FROM Folder
			WHERE userid = $1 AND name = $3
			AND NOT EXISTS (SELECT 1 FROM FolderChildren WHERE userid = $1 AND child = $2)
			ON CONFLICT (userid, parent, child) DO NOTHING
		`
		_, err = tx.ExecContext(ctx, query, u.UserId, folderID, models.RootFolder)
		if err != nil {
			return errFolderId, fmt.Errorf("failed to file folder under the root: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return errFolderId, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return folderID, nil
}

/*******************************************************************************
 * Content deletion
 ******************************************************************************/

// DeleteArticlesForUser deletes all articles earlier than the given timestamp
// and returns the number deleted. On error, -1 is returned for the number of
// articles deleted. Only articles that are read and not saved are deleted.
func (crdb *Crdb) DeleteArticlesForUser(u models.User, minTimestamp time.Time) (int64, error) {
	defer logElapsedTime(time.Now(), "DeleteArticlesForUser")

	query := `
		DELETE FROM Article 
		WHERE userid = $1 
		  AND read
		  AND not saved
		  AND (retrieved IS NULL OR retrieved < $2)
	`
	result, err := crdb.db.Exec(query, u.UserId, minTimestamp)
	if err != nil {
		return -1, fmt.Errorf("failed to delete articles: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return -1, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// DeleteArticlesByIdForUser deletes articles in the given list of IDs for the given user.
func (crdb *Crdb) DeleteArticlesByIdForUser(u models.User, ids []int64) error {
	defer logElapsedTime(time.Now(), "DeleteArticlesByIdForUser")

	query := `DELETE FROM Article WHERE userid = $1 AND id = ANY($2)`
	_, err := crdb.db.Exec(query, u.UserId, pq.Array(ids))
	return err
}

// TombstoneFeedForUser unsubscribes the user from a feed, reporting a feed that
// is not theirs, or is already unsubscribed from, as sql.ErrNoRows.
//
// The feed is marked rather than deleted. Deleting it means deleting every
// article it holds, which the request would otherwise wait on, and a fetch
// already in flight would be racing the delete. Marked, the feed and its
// articles drop out of every read a client can make, the fetcher's writes into
// it do nothing, and the garbage collector removes it on its next run.
func (crdb *Crdb) TombstoneFeedForUser(u models.User, feedId int64) error {
	defer logElapsedTime(time.Now(), "TombstoneFeedForUser")

	query := `UPDATE Feed SET deleted = now() WHERE userid = $1 AND id = $2 AND deleted IS NULL`
	res, err := crdb.db.Exec(query, u.UserId, feedId)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RestoreFeedByUrlForUser resubscribes the user to a feed with the given URL
// that they unsubscribed from and the garbage collector has not yet removed,
// returning it with the articles it had. A URL with no such feed is reported
// as sql.ErrNoRows.
func (crdb *Crdb) RestoreFeedByUrlForUser(u models.User, url string) (models.Feed, error) {
	defer logElapsedTime(time.Now(), "RestoreFeedByUrlForUser")

	var f models.Feed
	query := `
		UPDATE Feed SET deleted = NULL
		WHERE userid = $1 AND id = (
			SELECT id FROM Feed
			WHERE userid = $1 AND url = $2 AND deleted IS NOT NULL
			ORDER BY id
			LIMIT 1
		)
		RETURNING id, folder, title, description, url, link, latest, estimated_refresh_interval, titleoverridden
	`
	err := crdb.db.QueryRow(query, u.UserId, url).Scan(
		&f.ID, &f.FolderID, &f.Title, &f.Description, &f.URL, &f.Link, &f.Latest,
		&f.EstimatedRefreshInterval, &f.TitleOverridden)
	if err != nil {
		return models.Feed{}, err
	}
	return f, nil
}

// PurgeDeletedFeeds removes every feed unsubscribed from before the given time,
// along with its articles, returning how many of each were removed.
//
// One transaction and one cutoff, so that a feed unsubscribed from while this
// runs is either removed whole or left whole: never its row without its
// articles, which the foreign key would refuse anyway.
func (crdb *Crdb) PurgeDeletedFeeds(before time.Time) (int64, int64, error) {
	defer logElapsedTime(time.Now(), "PurgeDeletedFeeds")

	ctx, cancel := context.WithTimeout(context.Background(), maxOperationTime)
	defer cancel()
	tx, err := crdb.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSilent(tx)

	// Article's foreign key to Feed has no ON DELETE, so the articles go first.
	query := `
		DELETE FROM Article
		WHERE (userid, folder, feed) IN (SELECT userid, folder, id FROM Feed WHERE deleted < $1)
	`
	res, err := tx.ExecContext(ctx, query, before)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to delete articles: %w", err)
	}
	articles, err := res.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count deleted articles: %w", err)
	}

	res, err = tx.ExecContext(ctx, `DELETE FROM Feed WHERE deleted < $1`, before)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to delete feeds: %w", err)
	}
	feeds, err := res.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count deleted feeds: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return feeds, articles, nil
}

// ErrRootFolder reports an attempt to remove the root folder. It is where a
// feed goes when it is filed nowhere else, so every user has to have one.
var ErrRootFolder = errors.New("the root folder cannot be removed")

// DeleteFolderForUser removes one of the user's folders and returns how many
// feeds it held. A folder that is not theirs is reported as sql.ErrNoRows.
//
// The feeds in it move to the root folder rather than going with it: removing
// a way of filing feeds is not a request to unsubscribe from them. Their
// articles follow through the cascade on Article's key to Feed. A folder nested
// inside it moves up to the root in the same way.
//
// One transaction, so that a failure part-way leaves the folder as it was
// rather than half emptied.
func (crdb *Crdb) DeleteFolderForUser(u models.User, folderId int64) (int64, error) {
	defer logElapsedTime(time.Now(), "DeleteFolderForUser")

	ctx, cancel := context.WithTimeout(context.Background(), maxOperationTime)
	defer cancel()
	tx, err := crdb.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSilent(tx)

	var name string
	query := `SELECT name FROM Folder WHERE userid = $1 AND id = $2`
	if err = tx.QueryRowContext(ctx, query, u.UserId, folderId).Scan(&name); err != nil {
		return 0, err
	}
	if name == models.RootFolder {
		return 0, ErrRootFolder
	}

	var rootId int64
	query = `SELECT id FROM Folder WHERE userid = $1 AND name = $2`
	if err = tx.QueryRowContext(ctx, query, u.UserId, models.RootFolder).Scan(&rootId); err != nil {
		return 0, fmt.Errorf("failed to find the root folder: %w", err)
	}

	// Feeds unsubscribed from move too, since the folder is going, but are not
	// counted: they are not feeds the user would say the folder held.
	query = `
		WITH moved AS (
			UPDATE Feed SET folder = $3 WHERE userid = $1 AND folder = $2 RETURNING deleted
		)
		SELECT count(*) FROM moved WHERE deleted IS NULL
	`
	var moved int64
	if err = tx.QueryRowContext(ctx, query, u.UserId, folderId, rootId).Scan(&moved); err != nil {
		return 0, fmt.Errorf("failed to move feeds to the root folder: %w", err)
	}

	query = `
		INSERT INTO FolderChildren(userid, parent, child)
		SELECT userid, $3::INT8, child FROM FolderChildren WHERE userid = $1 AND parent = $2
		ON CONFLICT (userid, parent, child) DO NOTHING
	`
	if _, err = tx.ExecContext(ctx, query, u.UserId, folderId, rootId); err != nil {
		return 0, fmt.Errorf("failed to move subfolders to the root folder: %w", err)
	}

	query = `DELETE FROM FolderChildren WHERE userid = $1 AND (parent = $2 OR child = $2)`
	if _, err = tx.ExecContext(ctx, query, u.UserId, folderId); err != nil {
		return 0, fmt.Errorf("failed to unlink folder: %w", err)
	}

	query = `DELETE FROM Folder WHERE userid = $1 AND id = $2`
	if _, err = tx.ExecContext(ctx, query, u.UserId, folderId); err != nil {
		return 0, fmt.Errorf("failed to delete folder: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return moved, nil
}

/*******************************************************************************
 * Marking
 ******************************************************************************/

// Every statement that changes an article's read flag also maintains readat, so
// that "what became read since <time>" is answerable. Marking read records the
// current time unless one is already recorded, since re-marking a read article
// — which marking a whole feed or folder does to every article in it — is not a
// new read event. Marking unread clears it, leaving NULL to mean "not read",
// with the one exception of an article read before the column existed.

// MarkArticleForUser sets the mark status of `articleId` to `mark`.
func (crdb *Crdb) MarkArticleForUser(u models.User, articleId int64, mark models.MarkAction) error {
	defer logElapsedTime(time.Now(), "MarkArticleForUser")

	markType, value, err := mark.Parse()
	if err != nil {
		return fmt.Errorf("invalid mark action: %+v", mark)
	}

	var query string
	switch markType {
	case models.MarkTypeRead:
		query = `UPDATE Article SET read = $1, readat = CASE WHEN $1 THEN COALESCE(readat, now()) END WHERE userid = $2 AND id = $3`
	case models.MarkTypeSaved:
		query = `UPDATE Article SET saved = $1 WHERE userid = $2 AND id = $3`
	default:
		return fmt.Errorf("invalid mark type: %+v", mark)
	}

	_, err = crdb.db.Exec(query, value, u.UserId, articleId)
	return err
}

// MarkArticlesForUser sets the mark status of every article in `articleIds` to
// `mark` in a single statement. Returns the number of articles whose state was
// changed.
//
// Clients mark in bulk, batching hundreds of IDs into a single request, so
// marking per ID costs that many sequential round trips and leaves a failure
// partway through half-applied.
func (crdb *Crdb) MarkArticlesForUser(u models.User, articleIds []int64, mark models.MarkAction) (int64, error) {
	defer logElapsedTime(time.Now(), "MarkArticlesForUser")

	if len(articleIds) == 0 {
		return 0, nil
	}

	markType, value, err := mark.Parse()
	if err != nil {
		return 0, fmt.Errorf("invalid mark action: %+v", mark)
	}

	var query string
	switch markType {
	case models.MarkTypeRead:
		query = `UPDATE Article SET read = $1, readat = CASE WHEN $1 THEN COALESCE(readat, now()) END WHERE userid = $2 AND id = ANY($3)`
	case models.MarkTypeSaved:
		query = `UPDATE Article SET saved = $1 WHERE userid = $2 AND id = ANY($3)`
	default:
		return 0, fmt.Errorf("invalid mark type: %+v", mark)
	}

	result, err := crdb.db.Exec(query, value, u.UserId, pq.Array(articleIds))
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// MarkFeedForUser sets the mark status of all articles in `feedId` to `mark`.
// Returns the number of articles whose state was changed.
func (crdb *Crdb) MarkFeedForUser(u models.User, feedId int64, mark models.MarkAction) (int64, error) {
	defer logElapsedTime(time.Now(), "MarkFeedForUser")

	if mark != models.MarkActionRead {
		return 0, fmt.Errorf("feeds can only be marked as read")
	}

	_, value, err := mark.Parse()
	if err != nil {
		return 0, fmt.Errorf("invalid mark action: %+v", mark)
	}

	query := `UPDATE Article SET read = $1, readat = CASE WHEN $1 THEN COALESCE(readat, now()) END WHERE userid = $2 AND feed = $3`
	result, err := crdb.db.Exec(query, value, u.UserId, feedId)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// MarkFolderForUser sets the mark status of all articles in `folderId` to
// `mark`. An ID of 0 will mark all articles in all folders to the given status.
// Returns the number of articles whose state was changed.
func (crdb *Crdb) MarkFolderForUser(u models.User, folderId int64, mark models.MarkAction) (int64, error) {
	defer logElapsedTime(time.Now(), "MarkFolderForUser")

	if mark != models.MarkActionRead {
		return 0, fmt.Errorf("folders can only be marked as read")
	}

	_, value, err := mark.Parse()
	if err != nil {
		return 0, fmt.Errorf("invalid mark action: %+v", mark)
	}

	// With folderID = 0, mark everything as read.
	if folderId == 0 {
		query := `UPDATE Article SET read = $1, readat = CASE WHEN $1 THEN COALESCE(readat, now()) END WHERE userid = $2`
		result, err := crdb.db.Exec(query, value, u.UserId)
		if err != nil {
			return 0, fmt.Errorf("failed to update articles for all folders: %w", err)
		}
		n, _ := result.RowsAffected()
		return n, nil
	}

	// Enumerate all descendant folders of folderId in a recursive CTE and then
	// mark all articles in any of that set of folders (including the original
	// folder itself) in one update.
	query := `
		WITH RECURSIVE RecursiveFolders AS (
			SELECT child
			FROM FolderChildren
			WHERE userid = $1 AND parent = $2
			UNION ALL
			SELECT fc.child
			FROM FolderChildren fc
			INNER JOIN RecursiveFolders rf ON fc.parent = rf.child
			WHERE fc.userid = $1
		)
		UPDATE Article AS a
		SET read = $3, readat = CASE WHEN $3 THEN COALESCE(readat, now()) END
		WHERE a.userid = $1
		  AND (
			a.folder IN (SELECT child FROM RecursiveFolders)
			OR a.folder = $2
		  );
	`
	result, err := crdb.db.Exec(query, u.UserId, folderId, value)
	if err != nil {
		return 0, fmt.Errorf("failed to update articles for folder %d and its descendants: %w", folderId, err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

/*******************************************************************************
 * Metadata update
 ******************************************************************************/

// UpdateFeedMetadataForUser refreshes a feed's own metadata -- its title,
// description and site link -- with the values in the given models.Feed.
//
// This is what the fetcher writes, and it may be writing from a copy of the
// feed read before the user renamed it. So a title the user set is kept in the
// statement itself, not on the strength of the copy, and the flag recording it
// is left alone; RenameFeedForUser is the only thing that sets either.
func (crdb *Crdb) UpdateFeedMetadataForUser(u models.User, f models.Feed) error {
	defer logElapsedTime(time.Now(), "UpdateFeedMetadataForUser")

	query := `
		UPDATE Feed
		SET hash = CASE WHEN titleoverridden THEN hash ELSE $1 END,
		    title = CASE WHEN titleoverridden THEN title ELSE $2 END,
		    description = $3, link = $4
		WHERE userid = $5 AND id = $6 AND deleted IS NULL
	`
	_, err := crdb.db.Exec(
		query, f.Hash(), f.Title, f.Description, f.Link, u.UserId, f.ID)
	return err
}

// RenameFeedForUser gives a feed the title in the given models.Feed, and marks
// it as the user's own so that refreshing the feed's metadata keeps it.
func (crdb *Crdb) RenameFeedForUser(u models.User, f models.Feed) error {
	defer logElapsedTime(time.Now(), "RenameFeedForUser")

	query := `
		UPDATE Feed
		SET hash = $1, title = $2, titleoverridden = true
		WHERE userid = $3 AND id = $4 AND deleted IS NULL
	`
	_, err := crdb.db.Exec(query, f.Hash(), f.Title, u.UserId, f.ID)
	return err
}

// UpdateLatestTimeForFeedForUser sets the latest retrieval time for the given
// feed to the given timestamp.
func (crdb *Crdb) UpdateLatestTimeForFeedForUser(u models.User, id int64, latest time.Time) error {
	defer logElapsedTime(time.Now(), "UpdateLatestTimeForFeedForUser")

	query := `
		UPDATE Feed
		SET latest = $1
		WHERE userid = $2 AND id = $3 AND deleted IS NULL
	`
	_, err := crdb.db.Exec(query, latest, u.UserId, id)
	return err
}

// UpdateEstimatedRefreshIntervalForFeedForUser sets the estimated refresh interval (in seconds) for the given feed.
func (crdb *Crdb) UpdateEstimatedRefreshIntervalForFeedForUser(u models.User, id int64, interval int) error {
	defer logElapsedTime(time.Now(), "UpdateEstimatedRefreshIntervalForFeedForUser")

	query := `
		UPDATE Feed
		SET estimated_refresh_interval = $1
		WHERE userid = $2 AND id = $3 AND deleted IS NULL
	`
	_, err := crdb.db.Exec(query, interval, u.UserId, id)
	return err
}

// UpdateFolderForFeedForUser updates the folder of the given feed.
// The new `folderId` must already exist and is enforced by a foreign key
// constraint on the `Feed` folder.
func (crdb *Crdb) UpdateFolderForFeedForUser(u models.User, feedId int64, folderId int64) error {
	defer logElapsedTime(time.Now(), "UpdateFolderForFeedForUser")

	// The corresponding rows in the `Article` table will also be updated via the
	// `ON UPDATE CASCADE` setting of the foreign key constraint on that table, so
	// no need to directly update `Article` here.
	query := `UPDATE Feed SET folder = $1 WHERE userid = $2 and id = $3`
	_, err := crdb.db.Exec(query, folderId, u.UserId, feedId)
	return err
}

// ErrFolderNameTaken reports a rename onto a name another of the user's
// folders already has. Names are unique per user, and two folders answering to
// one name could not be told apart by anyone looking at the list.
var ErrFolderNameTaken = errors.New("folder name is already taken")

// RenameFolderForUser gives one of the user's folders a new name, reporting a
// folder that is not theirs as sql.ErrNoRows.
//
// The root is excluded by the statement itself rather than left to callers:
// its stored name is how it is found, so renaming it would leave the user with
// no root at all.
func (crdb *Crdb) RenameFolderForUser(u models.User, folderId int64, name string) error {
	defer logElapsedTime(time.Now(), "RenameFolderForUser")

	if err := models.ValidateFolderName(name); err != nil {
		return err
	}

	query := `UPDATE Folder SET name = $3 WHERE userid = $1 AND id = $2 AND name != $4`
	res, err := crdb.db.Exec(query, u.UserId, folderId, name, models.RootFolder)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code.Name() == "unique_violation" {
			return ErrFolderNameTaken
		}
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateArticleParsedContentForUser updates the parsed content column of the article.
func (crdb *Crdb) UpdateArticleParsedContentForUser(u models.User, articleID int64, parsed string) error {
	defer logElapsedTime(time.Now(), "UpdateArticleParsedContentForUser")

	query := `UPDATE Article SET parsed = $1 WHERE userid = $2 AND id = $3`
	_, err := crdb.db.Exec(query, parsed, u.UserId, articleID)
	return err
}

/*******************************************************************************
 * Content retrieval
 ******************************************************************************/

// GetFolderChildrenForUser returns a list of IDs corresponding to folders
// under the given folder ID.
func (crdb *Crdb) GetFolderChildrenForUser(u models.User, id int64) ([]int64, error) {
	defer logElapsedTime(time.Now(), "GetFolderChildrenForUser")

	var children []int64

	query := `SELECT child FROM FolderChildren WHERE userid = $1 AND parent = $2`
	rows, err := crdb.db.Query(query, u.UserId, id)
	defer closeSilent(rows)

	if err != nil {
		return children, err
	}

	var childID int64
	for rows.Next() {
		if err = rows.Scan(&childID); err != nil {
			return children, err
		}
		children = append(children, childID)
	}
	return children, err
}

// GetAllFoldersForUser returns a list of all folders in the database for the
// given user.
func (crdb *Crdb) GetAllFoldersForUser(u models.User) ([]models.Folder, error) {
	defer logElapsedTime(time.Now(), "GetAllFoldersForUser")

	// TODO: Consider returning a map[int64]models.Folder instead.
	var folders []models.Folder

	query := `SELECT id, name FROM Folder WHERE userid = $1`
	rows, err := crdb.db.Query(query, u.UserId)
	defer closeSilent(rows)

	if err != nil {
		return folders, err
	}

	for rows.Next() {
		f := models.Folder{}
		if err = rows.Scan(&f.ID, &f.Name); err != nil {
			return folders, err
		}
		folders = append(folders, f)
	}

	return folders, err
}

// GetAllFeedsForUser returns a list of all feeds in the database for the
// given user.
// GetFeedForUser returns a single feed belonging to the user.
//
// Ownership is part of the lookup rather than a check afterwards, so a feed
// belonging to somebody else is reported the same way as one that does not
// exist: sql.ErrNoRows. That distinction is not the caller's to make, and an
// endpoint that could make it would answer whether an ID is in use.
func (crdb *Crdb) GetFeedForUser(u models.User, feedId int64) (models.Feed, error) {
	defer logElapsedTime(time.Now(), "GetFeedForUser")

	var f models.Feed
	query := `
		SELECT id, folder, title, description, url, link, latest, estimated_refresh_interval, titleoverridden
		FROM Feed
		WHERE userid = $1 AND id = $2 AND deleted IS NULL
	`
	err := crdb.db.QueryRow(query, u.UserId, feedId).Scan(
		&f.ID, &f.FolderID, &f.Title, &f.Description, &f.URL, &f.Link, &f.Latest,
		&f.EstimatedRefreshInterval, &f.TitleOverridden)
	if err != nil {
		return models.Feed{}, err
	}
	return f, nil
}

// GetRootFolderForUser returns the folder that holds a user's feeds that are
// not in any folder of their own.
//
// The root is a real row rather than a null folder because a feed's folder is
// part of the key its articles are stored under, so every feed needs one. It is
// an implementation detail: its name is a sentinel, not something a user chose,
// and it should not reach a client as a folder they can see.
func (crdb *Crdb) GetRootFolderForUser(u models.User) (models.Folder, error) {
	defer logElapsedTime(time.Now(), "GetRootFolderForUser")

	var f models.Folder
	query := `SELECT id, name FROM Folder WHERE userid = $1 AND name = $2`
	if err := crdb.db.QueryRow(query, u.UserId, models.RootFolder).Scan(&f.ID, &f.Name); err != nil {
		return models.Folder{}, err
	}
	return f, nil
}

// GetFeedByUrlForUser returns the user's feed with the given URL, or
// sql.ErrNoRows if they are not subscribed to it.
//
// The user's own feeds are the search space, so this answers "am I already
// subscribed to this" without telling the asker anything about anyone else.
// Nothing stops one user holding the same URL twice -- the uniqueness
// constraint is on a hash that includes the title -- so the lowest ID is
// returned to keep repeated asks consistent.
func (crdb *Crdb) GetFeedByUrlForUser(u models.User, url string) (models.Feed, error) {
	defer logElapsedTime(time.Now(), "GetFeedByUrlForUser")

	var f models.Feed
	query := `
		SELECT id, folder, title, description, url, link, latest, estimated_refresh_interval, titleoverridden
		FROM Feed
		WHERE userid = $1 AND url = $2 AND deleted IS NULL
		ORDER BY id
		LIMIT 1
	`
	err := crdb.db.QueryRow(query, u.UserId, url).Scan(
		&f.ID, &f.FolderID, &f.Title, &f.Description, &f.URL, &f.Link, &f.Latest,
		&f.EstimatedRefreshInterval, &f.TitleOverridden)
	if err != nil {
		return models.Feed{}, err
	}
	return f, nil
}

// GetFolderForUser returns a single folder belonging to the user, reporting a
// folder that is not theirs as sql.ErrNoRows for the same reason as
// GetFeedForUser.
func (crdb *Crdb) GetFolderForUser(u models.User, folderId int64) (models.Folder, error) {
	defer logElapsedTime(time.Now(), "GetFolderForUser")

	var f models.Folder
	query := `SELECT id, name FROM Folder WHERE userid = $1 AND id = $2`
	if err := crdb.db.QueryRow(query, u.UserId, folderId).Scan(&f.ID, &f.Name); err != nil {
		return models.Folder{}, err
	}
	return f, nil
}

func (crdb *Crdb) GetAllFeedsForUser(u models.User) ([]models.Feed, error) {
	defer logElapsedTime(time.Now(), "GetAllFeedsForUser")

	var feeds []models.Feed

	query := `
		SELECT id, folder, title, description, url, link, latest, estimated_refresh_interval, titleoverridden
		FROM Feed
		WHERE userid = $1 AND deleted IS NULL
	`
	rows, err := crdb.db.Query(query, u.UserId)
	defer closeSilent(rows)

	if err != nil {
		return feeds, err
	}

	for rows.Next() {
		f := models.Feed{}
		if err = rows.Scan(&f.ID, &f.FolderID, &f.Title, &f.Description, &f.URL, &f.Link, &f.Latest, &f.EstimatedRefreshInterval, &f.TitleOverridden); err != nil {
			return feeds, err
		}
		feeds = append(feeds, f)
	}

	return feeds, err
}

// GetFeedsInFolderForUser returns a list of feeds directly under the given
// folder for the given user.
func (crdb *Crdb) GetFeedsInFolderForUser(u models.User, folderId int64) ([]models.Feed, error) {
	defer logElapsedTime(time.Now(), "GetFeedsInFolderForUser")

	var feeds []models.Feed

	query := `SELECT id, title, url FROM Feed WHERE userid = $1 AND folder = $2 AND deleted IS NULL`
	rows, err := crdb.db.Query(query, u.UserId, folderId)
	defer closeSilent(rows)

	if err != nil {
		return feeds, err
	}

	for rows.Next() {
		feed := models.Feed{}
		if err := rows.Scan(&feed.ID, &feed.Title, &feed.URL); err != nil {
			return feeds, err
		}
		feeds = append(feeds, feed)
	}
	return feeds, nil
}

// GetFeedsPerFolderForUser returns a map of folder ID to an array of feed IDs.
func (crdb *Crdb) GetFeedsPerFolderForUser(u models.User) (map[int64][]int64, error) {
	defer logElapsedTime(time.Now(), "GetFeedsPerFolderForUser")

	resp := map[int64][]int64{}

	query := `SELECT folder, id FROM Feed WHERE userid = $1 AND deleted IS NULL`
	rows, err := crdb.db.Query(query, u.UserId)
	defer closeSilent(rows)

	if err != nil {
		return resp, err
	}

	var folderID, feedID int64
	for rows.Next() {
		if err = rows.Scan(&folderID, &feedID); err != nil {
			return resp, err
		}
		resp[folderID] = append(resp[folderID], feedID)
	}

	return resp, err
}

// GetFolderFeedTreeForUser returns a root Folder object with associated feeds
// and recursively populated subfolders.
func (crdb *Crdb) GetFolderFeedTreeForUser(u models.User) (*models.Folder, error) {
	defer logElapsedTime(time.Now(), "GetFolderFeedTreeForUser")

	// TODO: Make this method into a transaction.

	// Determine the root ID
	root, err := crdb.GetRootFolderForUser(u)
	if err != nil {
		return nil, err
	}
	rootId := root.ID

	// Create a map from ID to Folder
	folders, err := crdb.GetAllFoldersForUser(u)
	if err != nil {
		return nil, fmt.Errorf("error getting all folder for user: %w", err)
	}

	folderMap := make(map[int64]*models.Folder)
	for id := range folders {
		folderMap[folders[id].ID] = &folders[id]
	}

	// Assemble the folder parent/child relationships
	query := `SELECT parent, child FROM FolderChildren WHERE userid = $1`
	folderChildren, err := crdb.db.Query(query, u.UserId)
	defer closeSilent(folderChildren)

	if err != nil {
		return nil, fmt.Errorf("failed to get folder hierarchy: %w", err)
	}

	for folderChildren.Next() {
		var parentID, childID int64
		if err := folderChildren.Scan(&parentID, &childID); err != nil {
			return nil, fmt.Errorf("failed to scan folder child relationship: %w", err)
		}
		if parent, ok := folderMap[parentID]; ok {
			if child, ok := folderMap[childID]; ok {
				parent.Folders = append(parent.Folders, *child)
			}
		}
	}
	if err = folderChildren.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over folder hierarchy: %w", err)
	}

	// Map all feeds to their respective folders
	feeds, err := crdb.GetAllFeedsForUser(u)
	if err != nil {
		return nil, fmt.Errorf("error getting all feeds for user: %w", err)
	}

	for _, f := range feeds {
		if folder, ok := folderMap[f.FolderID]; ok {
			folder.Feed = append(folder.Feed, f)
		}
	}

	rootFolder, ok := folderMap[rootId]
	if !ok {
		return nil, fmt.Errorf("root folder not found in folder map for user %s", u.UserId)
	}

	return rootFolder, nil
}

// GetAllFaviconsForUser returns a map of feed ID to a base64 representation of
// its favicon. Feeds with no favicons are not part of the returned map.
func (crdb *Crdb) GetAllFaviconsForUser(u models.User) (map[int64]string, error) {
	defer logElapsedTime(time.Now(), "GetAllFaviconsForUser")

	// TODO: Consider returning a Favicon model type.
	favicons := map[int64]string{}

	query := `
		SELECT id, mime, favicon
		FROM Feed
		WHERE userid = $1 AND favicon IS NOT NULL AND deleted IS NULL
	`
	rows, err := crdb.db.Query(query, u.UserId)
	defer closeSilent(rows)

	if err != nil {
		return favicons, err
	}

	var id int64
	var mime string
	var favicon string
	for rows.Next() {
		if err = rows.Scan(&id, &mime, &favicon); err != nil {
			return favicons, err
		}
		favicons[id] = fmt.Sprintf("%s;base64,%s", mime, favicon)
	}
	return favicons, err
}

// liveArticles restricts a read of Article to articles whose feed the user is
// still subscribed to. Every read a client can reach carries it: the articles
// of a feed unsubscribed from stay stored until the garbage collector removes
// them with the feed, so that re-adding it can bring them back. It names the
// user as $1, which every such query already binds.
const liveArticles = `feed NOT IN (SELECT id FROM Feed WHERE userid = $1 AND deleted IS NOT NULL)`

// articleMetaQuery is the one shape behind every filtered metadata read. The
// substituted fragments are constants chosen from the stream filter, never
// caller input; every value travels as a bound parameter.
var articleMetaQuery = template.Must(template.New("articleMeta").Parse(`
		SELECT id, feed, folder, date
		FROM Article
		WHERE userid = $1 AND id > $2 AND {{.Filter}} AND ` + liveArticles + `
		{{- with .SinceColumn}}
		  AND {{.}} > $4
		{{- end}}
		ORDER BY id LIMIT $3
	`))

// articleMetaQueryFragments names what articleMetaQuery substitutes.
type articleMetaQueryFragments struct {
	// Filter is the predicate selecting the stream's articles.
	Filter string
	// SinceColumn is the timestamp column a time bound applies to, or empty
	// for an unbounded query.
	SinceColumn string
}

// articleMetaFragments returns the query fragments for a stream filter.
//
// A time bound applies to whichever column records when an article entered the
// stream. For the read stream that is readat, which is NULL for an article read
// before the column existed; such an article is left out rather than dated by a
// guess, and the garbage collector retires the population within its keep
// window.
func articleMetaFragments(filter models.StreamFilter) (articleMetaQueryFragments, error) {
	switch filter {
	case models.StreamFilterRead:
		return articleMetaQueryFragments{Filter: "read", SinceColumn: "readat"}, nil
	case models.StreamFilterUnread:
		return articleMetaQueryFragments{Filter: "NOT read", SinceColumn: "date"}, nil
	case models.StreamFilterSaved:
		return articleMetaQueryFragments{Filter: "saved", SinceColumn: "date"}, nil
	case models.StreamFilterUnsaved:
		return articleMetaQueryFragments{Filter: "NOT saved", SinceColumn: "date"}, nil
	default:
		return articleMetaQueryFragments{}, fmt.Errorf("invalid filter: %+v", filter)
	}
}

// GetArticleMetaWithFilterForUser returns a list of <=`limit` articles with
// `filter` within `cursor`. Only metadata fields are returned, not content.
func (crdb *Crdb) GetArticleMetaWithFilterForUser(u models.User, filter models.StreamFilter, limit int, cursor models.StreamCursor) ([]models.ArticleMeta, error) {
	defer logElapsedTime(time.Now(), "GetUnreadArticleMetaForUser")

	var articles []models.ArticleMeta
	var rows *sql.Rows

	if limit == -1 {
		limit = MaxFetchedRows
	}
	sinceID := cursor.SinceID
	if sinceID == -1 {
		sinceID = 0
	}

	fragments, err := articleMetaFragments(filter)
	if err != nil {
		return articles, err
	}

	args := []any{u.UserId, sinceID, limit}
	if cursor.Since.IsZero() {
		fragments.SinceColumn = ""
	} else {
		args = append(args, cursor.Since)
	}

	var query strings.Builder
	if err = articleMetaQuery.Execute(&query, fragments); err != nil {
		return articles, fmt.Errorf("failed to build article metadata query: %w", err)
	}

	rows, err = crdb.db.Query(query.String(), args...)
	defer closeSilent(rows)

	if err != nil {
		return articles, err
	}

	for rows.Next() {
		a := models.ArticleMeta{}
		if err = rows.Scan(
			&a.ID, &a.FeedID, &a.FolderID, &a.Date); err != nil {
			return articles, err
		}
		articles = append(articles, a)
	}
	return articles, err
}

// GetArticleContentsForUser returns the text of articles owned by the user,
// in ID order starting after the given ID, with only the fields that can carry
// rewritten URLs populated.
//
// Paged rather than returned whole because the content of every article at once
// is far more than needs to be held to rewrite one.
func (crdb *Crdb) GetArticleContentsForUser(u models.User, afterID int64, limit int) ([]models.Article, error) {
	defer logElapsedTime(time.Now(), "GetArticleContentsForUser")

	var articles []models.Article

	query := `
		SELECT id, summary, content FROM Article
		WHERE userid = $1 AND id > $2
		ORDER BY id
		LIMIT $3`
	rows, err := crdb.db.Query(query, u.UserId, afterID, limit)
	defer closeSilent(rows)

	if err != nil {
		return articles, err
	}

	for rows.Next() {
		a := models.Article{}
		if err = rows.Scan(&a.ID, &a.Summary, &a.Content); err != nil {
			return articles, err
		}
		articles = append(articles, a)
	}

	return articles, rows.Err()
}

// UpdateArticleContentForUser replaces the text of a single article.
func (crdb *Crdb) UpdateArticleContentForUser(u models.User, id int64, summary string, content string) error {
	defer logElapsedTime(time.Now(), "UpdateArticleContentForUser")

	query := `UPDATE Article SET summary = $1, content = $2 WHERE userid = $3 AND id = $4`
	_, err := crdb.db.Exec(query, summary, content, u.UserId, id)
	return err
}

// GetArticlesForUser returns articles from the specified list.
func (crdb *Crdb) GetArticlesForUser(u models.User, ids []int64) ([]models.Article, error) {
	defer logElapsedTime(time.Now(), "GetArticlesForUser")

	var articles []models.Article
	var rows *sql.Rows
	var err error

	query := `
		SELECT id, feed, folder, title, summary, content, parsed, link, date
		FROM Article
		WHERE userid = $1 AND id = ANY($2) AND ` + liveArticles
	rows, err = crdb.db.Query(query, u.UserId, pq.Array(ids))
	defer closeSilent(rows)

	if err != nil {
		return articles, err
	}

	for rows.Next() {
		a := models.Article{}
		if err = rows.Scan(
			&a.ID, &a.FeedID, &a.FolderID, &a.Title, &a.Summary, &a.Content, &a.Parsed, &a.Link, &a.Date); err != nil {
			return articles, err
		}
		articles = append(articles, a)
	}
	return articles, err
}

// GetArticlesWithFilterForUser returns a list of <=`limit` articles with
// `filter` after `sinceId`.
func (crdb *Crdb) GetArticlesWithFilterForUser(u models.User, filter models.StreamFilter, limit int, sinceID int64) ([]models.Article, error) {
	defer logElapsedTime(time.Now(), "GetUnreadArticlesForUser")

	var articles []models.Article
	var rows *sql.Rows
	var err error

	if limit == -1 {
		limit = MaxFetchedRows
	}
	if sinceID == -1 {
		sinceID = 0
	}

	fragments, err := articleMetaFragments(filter)
	if err != nil {
		return articles, err
	}
	query := `
		SELECT id, feed, folder, title, summary, content, parsed, link, date
		FROM Article
		WHERE userid = $1 AND id > $2 AND ` + fragments.Filter + ` AND ` + liveArticles + `
		ORDER BY id LIMIT $3
	`

	rows, err = crdb.db.Query(query, u.UserId, sinceID, limit)
	defer closeSilent(rows)

	if err != nil {
		return articles, err
	}

	for rows.Next() {
		a := models.Article{}
		if err = rows.Scan(
			&a.ID, &a.FeedID, &a.FolderID, &a.Title, &a.Summary, &a.Content, &a.Parsed, &a.Link, &a.Date); err != nil {
			return articles, err
		}
		articles = append(articles, a)
	}
	return articles, err
}

// GetArticlesForFeedForUser returns a list of articles for the
// given feed ID and user.
func (crdb *Crdb) GetArticlesForFeedForUser(u models.User, feedId int64) ([]models.Article, error) {
	defer logElapsedTime(time.Now(), "GetUnreadArticlesForFeedForUser")

	var articles []models.Article
	var rows *sql.Rows
	var err error

	query := `
		SELECT id, feed, folder, title, summary, content, parsed, link, read, saved, date
		FROM Article
		WHERE userid = $1 AND feed = $2
	`
	rows, err = crdb.db.Query(query, u.UserId, feedId)
	defer closeSilent(rows)

	if err != nil {
		return articles, err
	}

	for rows.Next() {
		a := models.Article{}
		if err = rows.Scan(
			&a.ID, &a.FeedID, &a.FolderID, &a.Title, &a.Summary, &a.Content, &a.Parsed, &a.Link, &a.Read, &a.Saved, &a.Date); err != nil {
			return articles, err
		}
		articles = append(articles, a)
	}
	return articles, err
}

/*******************************************************************************
 * OPML
 ******************************************************************************/

// ImportOpmlForUser inserts folders from the given OPML object into the
// database for the given user.
func (crdb *Crdb) ImportOpmlForUser(u models.User, opml *opml.Opml) error {
	root := opml.Folders
	rootID, err := crdb.InsertFolderForUser(u, root, 0)
	if err != nil {
		return err
	}
	root.ID = rootID

	return importChildrenForUser(crdb, u, root)
}

func importChildrenForUser(crdb *Crdb, u models.User, parent models.Folder) error {
	var err error
	for _, f := range parent.Feed {
		feedID, err := crdb.InsertFeedForUser(u, f, parent.ID)
		if err != nil {
			return err
		}
		f.ID = feedID
	}

	for _, child := range parent.Folders {
		childID, err := crdb.InsertFolderForUser(u, child, parent.ID)
		if err != nil {
			return err
		}
		child.ID = childID

		if err = importChildrenForUser(crdb, u, child); err != nil {
			return err
		}
	}
	return err
}

/*******************************************************************************
 * Helper methods
 ******************************************************************************/

func closeSilent(rows *sql.Rows) {
	if rows == nil {
		log.Warningf("WARNING: Rows object is nil, nothing to close.")
		return
	}

	err := rows.Close()
	if err != nil {
		log.Warningf("Failed to close rows: %+v", rows)
	}
}

func rollbackSilent(tx *sql.Tx) {
	if tx == nil {
		log.Warningf("WARNING: Transaction object is nil, nothing to rollback.")
		return
	}
	_ = tx.Rollback()
}
