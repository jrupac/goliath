-- The current schema, in full, and safe to apply to a database in any state.
--
-- Two rules keep it that way. Tables appear in dependency order: a table's
-- foreign keys must name tables the file has already created, so a new table
-- goes after everything it references, not next to whatever it is related to.
-- And every statement is conditional, indexes included — an index created
-- without a name is named after its columns with a numeric suffix if that name
-- is taken, so an anonymous CREATE INDEX does not fail on a second run, it
-- silently builds a duplicate.

CREATE DATABASE IF NOT EXISTS Goliath;

-- User "goliath" must already exist.
GRANT ALL ON DATABASE Goliath TO goliath;

SET DATABASE TO Goliath;

CREATE TABLE IF NOT EXISTS UserTable
(
    -- Key columns
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username STRING NOT NULL,
    -- Data columns
    key      STRING NOT NULL,
    hashpass STRING
);

-- A user is looked up by either credential, and both lookups read both
-- columns, so each index carries the other. These also carry the uniqueness of
-- username and key, which is why neither column declares it inline.
CREATE UNIQUE INDEX IF NOT EXISTS usertable_username_key
    ON UserTable (username) STORING (key);
CREATE UNIQUE INDEX IF NOT EXISTS usertable_key_key
    ON UserTable (key) STORING (username);

CREATE TABLE IF NOT EXISTS Session
(
    -- Key columns
    id        UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    -- The bearer token is never stored; only its digest is, so that a leaked
    -- table (or database dump) does not hand over live sessions. Uniqueness is
    -- enforced by the covering index below rather than here, to avoid keeping
    -- two indexes on the same column.
    tokenhash BYTES       NOT NULL,
    userid    UUID        NOT NULL,
    -- Data columns
    scheme    STRING      NOT NULL,
    created   TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Expiry slides: a session is valid until it has gone unused for the
    -- configured idle window, so an actively syncing client never expires.
    lastseen  TIMESTAMPTZ NOT NULL DEFAULT now(),
    useragent STRING      NOT NULL DEFAULT '',
    CONSTRAINT session_userid_fkey
        FOREIGN KEY (userid)
            REFERENCES UserTable (id)
            ON DELETE CASCADE
);

-- The authentication path looks a session up by digest on every request and
-- reads only these columns, so serve it entirely from the index.
CREATE UNIQUE INDEX IF NOT EXISTS session_tokenhash_idx ON Session (tokenhash)
    STORING (userid, scheme, created, lastseen, useragent);

-- Listing a user's sessions, and revoking all of them at once.
CREATE INDEX IF NOT EXISTS session_userid_idx ON Session (userid);

-- Sweeping sessions that have gone idle past the expiry window.
CREATE INDEX IF NOT EXISTS session_lastseen_idx ON Session (lastseen);

CREATE TABLE IF NOT EXISTS UserPrefs
(
    -- Key columns
    userid     UUID NOT NULL PRIMARY KEY,
    -- Data columns
    mute_words STRING[],
    CONSTRAINT userprefs_userid_fkey
        FOREIGN KEY (userid)
            REFERENCES UserTable (id)
            ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS Folder
(
    -- Key columns
    userid UUID   NOT NULL REFERENCES UserTable (id),
    id     SERIAL NOT NULL UNIQUE,
    PRIMARY KEY (userid, id),
    -- Data columns
    name STRING,
    CONSTRAINT unique_userid_name
        UNIQUE (userid, name)
);

CREATE TABLE IF NOT EXISTS FolderChildren
(
    -- Key columns
    userid UUID NOT NULL REFERENCES UserTable (id),
    parent INT  NOT NULL REFERENCES Folder (id),
    child  INT  NOT NULL REFERENCES Folder (id),
    PRIMARY KEY (userid, parent, child)
);

CREATE TABLE IF NOT EXISTS Feed
(
    -- Key columns
    userid      UUID   NOT NULL,
    folder      INT    NOT NULL,
    id          SERIAL NOT NULL UNIQUE,
    PRIMARY KEY (userid, folder, id),
    CONSTRAINT fk_folder_cascade
        FOREIGN KEY (userid, folder)
            REFERENCES Folder
            ON UPDATE CASCADE,
    -- Metadata columns
    hash   STRING,
    -- Data columns
    title       STRING,
    description STRING,
    url         STRING,
    link        STRING,
    -- MIME type of the favicon
    mime        STRING,
    -- Base64 encoding of favicon
    favicon     STRING,
    -- Latest timestamp of articles in this feed
    latest TIMESTAMPTZ DEFAULT CAST(0 AS TIMESTAMPTZ),
    -- Estimated interval between feed fetches (in seconds)
    estimated_refresh_interval INT DEFAULT 600,
    CONSTRAINT unique_userid_hash
        UNIQUE (userid, hash)
);

CREATE
    INDEX IF NOT EXISTS feed_userid_idx
    ON Feed (userid)
    STORING (title, description, url, link, latest);

CREATE TABLE IF NOT EXISTS UserUnmuteFeeds
(
    -- Key columns
    userid UUID NOT NULL,
    feedid INT  NOT NULL,
    PRIMARY KEY (userid, feedid),
    CONSTRAINT fk_user
        FOREIGN KEY (userid)
            REFERENCES UserTable (id)
            ON DELETE CASCADE,
    CONSTRAINT fk_feed
        FOREIGN KEY (feedid)
            REFERENCES Feed (id)
            ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS RetrievalCache
(
    -- Key columns
    userid UUID NOT NULL,
    feedid INT  NOT NULL,
    -- Data columns
    cache  STRING,
    PRIMARY KEY (userid, feedid),
    CONSTRAINT fk_user
        FOREIGN KEY (userid)
            REFERENCES UserTable (id)
            ON DELETE CASCADE,
    CONSTRAINT fk_feed
        FOREIGN KEY (feedid)
            REFERENCES Feed (id)
            ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS Article
(
    -- Key columns
    userid    UUID   NOT NULL,
    folder    INT    NOT NULL,
    feed      INT    NOT NULL,
    id        SERIAL NOT NULL UNIQUE,
    PRIMARY KEY (userid, folder, feed, id),
    CONSTRAINT fk_feed_folder_cascade
        FOREIGN KEY (userid, folder, feed)
            REFERENCES Feed
            ON UPDATE CASCADE,
    -- Metadata columns
    hash      STRING,
    -- Data columns
    title     STRING,
    summary   STRING,
    content   STRING,
    parsed    STRING,
    link      STRING,
    read      BOOL,
    saved BOOL DEFAULT false,
    -- Publication timestamp
    date      TIMESTAMPTZ,
    -- Retrieval timestamp
    retrieved TIMESTAMPTZ,
    -- Timestamp at which the article was marked read; NULL while unread
    readat    TIMESTAMPTZ,
    CONSTRAINT unique_userid_feed_hash
        UNIQUE (userid, feed, hash)
);

CREATE
    UNIQUE INDEX IF NOT EXISTS article_idx_read_key
    ON Article (id) STORING (read);

CREATE
    INDEX IF NOT EXISTS article_userid_id_read_idx
    ON Article (userid, id, read)
    STORING (title, summary, content, parsed, link, date);

CREATE
    INDEX IF NOT EXISTS article_idx_readat
    ON Article (userid, readat) STORING (read, date);

CREATE TABLE IF NOT EXISTS UserFeedMuteRegexes
(
    userid UUID NOT NULL,
    feedid INT  NOT NULL,
    regex  STRING NOT NULL,
    PRIMARY KEY (userid, feedid, regex),
    CONSTRAINT fk_user
        FOREIGN KEY (userid)
            REFERENCES UserTable (id)
            ON DELETE CASCADE,
    CONSTRAINT fk_feed
        FOREIGN KEY (feedid)
            REFERENCES Feed (id)
            ON DELETE CASCADE
);

-- The application connects as `goliath`, but a table is owned by whoever ran
-- this file, and a grant on the database does not reach the tables in it. So
-- every table needs a grant, and this has to be the last statement here: it
-- covers what exists when it runs, not what a later statement creates.
--
-- A migration that adds a table carries its own GRANT for the same reason.
-- Development hides the omission, because restoring a database grants on
-- everything afterwards; the first environment to run such a migration on its
-- own is the one that breaks.
GRANT ALL ON SCHEMA Goliath.public TO goliath;
GRANT ALL ON ALL TABLES IN SCHEMA Goliath.public TO goliath;
