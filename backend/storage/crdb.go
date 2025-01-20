package storage

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/opml"
	"github.com/lib/pq"
)

const (
	dialect            = "postgres"
	maxFetchedRows     = 10000
	slowOpLogThreshold = 50 * time.Millisecond
	retryBackoff       = 1 * time.Second
	maxOpTime = 30 * time.Second
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
func (crdb *Crdb) GetUserByKey(key string) (models.User, error) {
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

/*******************************************************************************
 * User preferences
 ******************************************************************************/

// ListMuteWordsForUser returns a list of mute words for the given user.
func (crdb *Crdb) ListMuteWordsForUser(u models.User) ([]string, error) {
	defer logElapsedTime(time.Now(), "ListMuteWordsForUser")

	var words []string

	query := `SELECT word FROM MuteWords WHERE userid = $1`
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
		ON CONFLICT (id) DO UPDATE
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

/*******************************************************************************
 * Retrieval cache
 ******************************************************************************/

// GetAllRetrievalCaches retrieves the cache for all users.
func (crdb *Crdb) GetAllRetrievalCaches() (map[string]string, error) {
	defer logElapsedTime(time.Now(), "GetAllRetrievalCaches")

	query := `SELECT userid, cache FROM RetrievalCache`
	rows, err := crdb.db.Query(query)
	defer closeSilent(rows)

	if err != nil {
		return nil, err
	}

	ret := map[string]string{}

	for rows.Next() {
		var id string
		var cache string
		if err = rows.Scan(&id, &cache); err != nil {
			return nil, err
		}
		ret[id] = cache
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ret, nil
}

// PersistAllRetrievalCaches writes the retrieval caches for all users.
func (crdb *Crdb) PersistAllRetrievalCaches(entries map[string][]byte) error {
	defer logElapsedTime(time.Now(), "PersistAllRetrievalCaches")

	ctx, cancel := context.WithTimeout(context.Background(), maxOpTime)
	defer cancel()
	tx, err := crdb.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSilent(tx)

	query := ` UPSERT INTO RetrievalCache (userid, cache) VALUES ($1, $2) `
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}

	for id, cache := range entries {
		// TODO: Change to BYTE type for this column instead of string encoding
		encodedCache := base64.StdEncoding.EncodeToString(cache)
		_, err = stmt.ExecContext(ctx, id, encodedCache)
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

// InsertArticleForUser inserts the given article object into the database.
func (crdb *Crdb) InsertArticleForUser(u models.User, a models.Article) error {
	defer logElapsedTime(time.Now(), "InsertArticleForUser")

	query := `
		INSERT INTO Article (userid, folder, feed, hash, title, summary, content, parsed, link, read, date, retrieved)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (userid, feed, hash) DO NOTHING
		RETURNING id
	`
	err := crdb.db.QueryRow(query,
		u.UserId, a.FolderID, a.FeedID, a.Hash(), a.Title, a.Summary, a.Content, a.Parsed, a.Link, a.Read, a.Date, a.Retrieved,
	).Scan(&a.ID)

	if err != nil {
		// If no rows were returned, it means a duplicate was found
		if errors.Is(err, sql.ErrNoRows) {
			log.V(2).Infof("Duplicate article entry, skipping (hash): %s", a.Hash())
			return nil
		}
		return fmt.Errorf("failed to insert article: %w", err)
	}

	return nil
}

// InsertFaviconForUser inserts the given favicon and associated metadata into
// the database.
func (crdb *Crdb) InsertFaviconForUser(u models.User, folderId int64, feedId int64, mime string, img []byte) error {
	defer logElapsedTime(time.Now(), "InsertFaviconForUser")

	// TODO: Consider wrapping this into a Favicon model type.
	// Convert to a base64 encoded string before inserting
	h := base64.StdEncoding.EncodeToString(img)

	query := `
		UPDATE Feed 
		SET favicon = $1, mime = $2 
		WHERE userid = $3 AND folder = $4 AND id = $5
		  AND EXISTS (SELECT 1 FROM Feed WHERE userid = $3 AND folder = $4 AND id = $5)
	`
	result, err := crdb.db.Exec(query, h, mime, u.UserId, folderId, feedId)
	if err != nil {
		return fmt.Errorf("failed to update favicon: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("original feed not found for user %s, folder %d, feed %d", u.UserId, folderId, feedId)
	}

	return nil
}

// InsertFeedForUser inserts a new feed into the database. If `folderId` is 0,
// the feed is assumed to be a top-level entry. Otherwise, the feed will be
// nested under the folder with that ID. If the root folder does not exist,
// returns -1 as the feed ID.
func (crdb *Crdb) InsertFeedForUser(u models.User, f models.Feed, folderId int64) (int64, error) {
	defer logElapsedTime(time.Now(), "InsertFeedForUser")

	var feedID int64

	// If the feed is assumed to be a top-level entry, determine the ID of the
	// root folder that it actually is under.
	if folderId == 0 {
		query := `SELECT id FROM Folder WHERE userid = $1 AND name = $2`
		err := crdb.db.QueryRow(query, u.UserId, models.RootFolder).Scan(&folderId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return -1, fmt.Errorf("root folder (%s) not found for user %s", models.RootFolder, u.UserId)
			} else {
				return -1, fmt.Errorf("failed to get root folder ID: %w", err)
			}
		}
	}

	query := `
		INSERT INTO Feed(userid, folder, hash, title, description, url, link)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT(userid, hash) DO UPDATE SET
			folder = excluded.folder,
			title = excluded.title,
			description = excluded.description,
			url = excluded.url,
			link = excluded.link
		RETURNING id
	`
	err := crdb.db.QueryRow(query, u.UserId, folderId, f.Hash(), f.Title, f.Description, f.URL, f.Link).Scan(&feedID)
	return feedID, err
}

// InsertFolderForUser inserts a new folder into the database. If `parentId` is
// 0, the folder is assumed to be the root folder. Otherwise, the folder will be
// nested under the folder with that ID. On error, -1 is returned for the folder
// ID.
func (crdb *Crdb) InsertFolderForUser(u models.User, f models.Folder, parentId int64) (int64, error) {
	defer logElapsedTime(time.Now(), "InsertFolderForUser")

	errFolderId := int64(-1)

	ctx, cancel := context.WithTimeout(context.Background(), maxOpTime)
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

	// If the parentID is not the root, update the FolderChildren mapping.
	// If the parentID is root, there is nothing to update.
	if parentId != 0 {
		query = `
			INSERT INTO FolderChildren(userid, parent, child) 
			VALUES($1, $2, $3)
			ON CONFLICT (userid, parent, child) DO NOTHING
		`
		_, err = tx.ExecContext(ctx, query, u.UserId, parentId, folderID)
		if err != nil {
			return errFolderId, fmt.Errorf("failed to insert into FolderChildren: %w", err)
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
// articles deleted.
func (crdb *Crdb) DeleteArticlesForUser(u models.User, minTimestamp time.Time) (int64, error) {
	defer logElapsedTime(time.Now(), "DeleteArticlesForUser")

	query := `
		DELETE FROM Article 
		WHERE userid = $1 
		  AND read 
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

// DeleteFeedForUser deletes the specified feed and all articles under that feed.
func (crdb *Crdb) DeleteFeedForUser(u models.User, feedId int64, folderId int64) error {
	defer logElapsedTime(time.Now(), "DeleteFeedForUser")

	// TODO: Add an `ON DELETE CASCADE` constraint to `Article` to simplify.

	ctx, cancel := context.WithTimeout(context.Background(), maxOpTime)
	defer cancel()
	tx, err := crdb.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSilent(tx)

	query := `DELETE FROM Article WHERE userid = $1 AND folder = $2 AND feed = $3`
	_, err = tx.ExecContext(ctx, query, u.UserId, folderId, feedId)
	if err != nil {
		return fmt.Errorf("failed to delete articles: %w", err)
	}

	query = `DELETE FROM Feed WHERE userid = $1 AND folder = $2 AND id = $3`
	_, err = tx.ExecContext(ctx, query, u.UserId, folderId, feedId)
	if err != nil {
		return fmt.Errorf("failed to delete feed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

/*******************************************************************************
 * Marking
 ******************************************************************************/

// MarkArticleForUser sets the read status of the given article to the given
// status.
func (crdb *Crdb) MarkArticleForUser(u models.User, articleId int64, status string) error {
	defer logElapsedTime(time.Now(), "MarkArticleForUser")

	// TODO: Consider creating a type for the status string.
	state, err := parseState(status)
	if err != nil {
		return err
	}

	query := `UPDATE Article SET read = $1 WHERE userid = $2 AND id = $3`
	_, err = crdb.db.Exec(query, state, u.UserId, articleId)
	return err
}

// MarkFeedForUser sets the read status of all articles in the given feed to the
// given status.
func (crdb *Crdb) MarkFeedForUser(u models.User, feedId int64, status string) error {
	defer logElapsedTime(time.Now(), "MarkFeedForUser")

	// TODO: Consider creating a type for the status string.
	state, err := parseState(status)
	if err != nil {
		return err
	}

	query := `UPDATE Article SET read = $1 WHERE userid = $2 AND feed = $3`
	_, err = crdb.db.Exec(query, state, u.UserId, feedId)
	return err
}

// MarkFolderForUser sets the read status of all articles in the given folder to
// the given status. An ID of 0 will mark all articles in all folders to the
// given status.
func (crdb *Crdb) MarkFolderForUser(u models.User, folderId int64, status string) error {
	defer logElapsedTime(time.Now(), "MarkFolderForUser")

	// TODO: Consider creating a type for the status string.
	state, err := parseState(status)
	if err != nil {
		return err
	}

	// With folderID = 0, just mark everything as read.
	if folderId == 0 {
		query := `UPDATE Article SET read = $1 WHERE userid = $2`
		_, err = crdb.db.Exec(query, state, u.UserId)
		if err != nil {
			return fmt.Errorf("failed to update articles for all folders: %w", err)
		}
	} else {
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
			SET read = $3
			WHERE a.userid = $1
			  AND (
				a.folder IN (SELECT child FROM RecursiveFolders)
				OR a.folder = $2
			  );
		`
		_, err = crdb.db.Exec(query, u.UserId, folderId, state)
		if err != nil {
			return fmt.Errorf("failed to update articles for folder %d and its descendants: %w", folderId, err)
		}
	}

	return nil
}

/*******************************************************************************
 * Metadata update
 ******************************************************************************/

// UpdateFeedMetadataForUser updates various fields for the row corresponding to
// given models.Feed object with the values in that object.
func (crdb *Crdb) UpdateFeedMetadataForUser(u models.User, f models.Feed) error {
	defer logElapsedTime(time.Now(), "UpdateFeedMetadataForUser")

	query := `
		UPDATE Feed
		SET hash = $1, title = $2, description = $3, link = $4
		WHERE userid = $5 AND folder = $6 AND id = $7
	`
	_, err := crdb.db.Exec(
		query, f.Hash(), f.Title, f.Description, f.Link, u.UserId, f.FolderID, f.ID)
	return err
}

// UpdateLatestTimeForFeedForUser sets the latest retrieval time for the given
// feed to the given timestamp.
func (crdb *Crdb) UpdateLatestTimeForFeedForUser(u models.User, folderId int64, id int64, latest time.Time) error {
	defer logElapsedTime(time.Now(), "UpdateLatestTimeForFeedForUser")

	query := `
		UPDATE Feed
		SET latest = $1
		WHERE userid = $2 AND folder = $3 AND id = $4
	`
	_, err := crdb.db.Exec(query, latest, u.UserId, folderId, id)
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
func (crdb *Crdb) GetAllFeedsForUser(u models.User) ([]models.Feed, error) {
	defer logElapsedTime(time.Now(), "GetAllFeedsForUser")

	var feeds []models.Feed

	query := `
		SELECT id, folder, title, description, url, link, latest
		FROM Feed
		WHERE userid = $1
	`
	rows, err := crdb.db.Query(query, u.UserId)
	defer closeSilent(rows)

	if err != nil {
		return feeds, err
	}

	for rows.Next() {
		f := models.Feed{}
		if err = rows.Scan(&f.ID, &f.FolderID, &f.Title, &f.Description, &f.URL, &f.Link, &f.Latest); err != nil {
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

	query := `SELECT id, title, url FROM Feed WHERE userid = $1 AND folder = $2`
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

	query := `SELECT folder, id FROM Feed WHERE userid = $1`
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
// and recursively populated sub-folders.
func (crdb *Crdb) GetFolderFeedTreeForUser(u models.User) (*models.Folder, error) {
	defer logElapsedTime(time.Now(), "GetFolderFeedTreeForUser")

	// TODO: Make this method into a transaction.

	var rootId int64

	// First determine the root ID
	query := `SELECT id from Folder WHERE userid = $1 AND name = $2`
	err := crdb.db.QueryRow(query, u.UserId, models.RootFolder).Scan(&rootId)
	if err != nil {
		return nil, err
	}

	// Create map from ID to Folder
	folders, err := crdb.GetAllFoldersForUser(u)
	if err != nil {
		return nil, fmt.Errorf("error getting all folder for user: %w", err)
	}

	folderMap := make(map[int64]*models.Folder)
	for id := range folders {
		folderMap[folders[id].ID] = &folders[id]
	}

	// Assemble the folder parent/child relationships
	query = `SELECT parent, child FROM FolderChildren WHERE userid = $1`
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
		WHERE userid = $1 AND favicon IS NOT NULL
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

// GetUnreadArticleMetaForUser returns a list of at most the given limit of
// articles after the given ID. Only metadata fields are returned, not content.
func (crdb *Crdb) GetUnreadArticleMetaForUser(u models.User, limit int, sinceID int64) ([]models.Article, error) {
	defer logElapsedTime(time.Now(), "GetUnreadArticlesForUser")

	var articles []models.Article
	var rows *sql.Rows
	var err error

	if limit == -1 {
		limit = maxFetchedRows
	}
	if sinceID == -1 {
		sinceID = 0
	}

	query := `
		SELECT id, feed, folder, date
		FROM Article
		WHERE userid = $1 AND id > $2 AND NOT read
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
			&a.ID, &a.FeedID, &a.FolderID, &a.Date); err != nil {
			return articles, err
		}
		articles = append(articles, a)
	}
	return articles, err
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
		WHERE userid = $1 AND id = ANY($2)
	`
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

// GetUnreadArticlesForUser returns a list of at most the given limit of
// articles after the given ID.
func (crdb *Crdb) GetUnreadArticlesForUser(u models.User, limit int, sinceID int64) ([]models.Article, error) {
	defer logElapsedTime(time.Now(), "GetUnreadArticlesForUser")

	var articles []models.Article
	var rows *sql.Rows
	var err error

	if limit == -1 {
		limit = maxFetchedRows
	}
	if sinceID == -1 {
		sinceID = 0
	}

	query := `
		SELECT id, feed, folder, title, summary, content, parsed, link, date
		FROM Article
		WHERE userid = $1 AND id > $2 AND NOT read
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
		SELECT id, feed, folder, title, summary, content, parsed, link, read, date
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
			&a.ID, &a.FeedID, &a.FolderID, &a.Title, &a.Summary, &a.Content, &a.Parsed, &a.Link, &a.Read, &a.Date); err != nil {
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

func parseState(status string) (bool, error) {
	var state bool
	switch status {
	case "saved", "unsaved":
		// TODO: Considering adding support for saving articles.
		return false, errors.New("Unsupported status: %s" + status)
	case "read":
		state = true
	case "unread":
		state = false
	}
	return state, nil
}

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
	err := tx.Rollback()
	if err != nil {
		log.Warningf("Failed to rollback transaction: %+v", tx)
	}
}
