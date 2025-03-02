CREATE DATABASE IF NOT EXISTS Goliath;

-- User "goliath" must already exist.
GRANT ALL ON DATABASE Goliath TO goliath;

SET DATABASE TO Goliath;

CREATE TABLE IF NOT EXISTS UserTable
(
    -- Key columns
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username STRING NOT NULL UNIQUE,
    -- Data columns
    key      STRING NOT NULL UNIQUE,
    hashpass STRING
);

CREATE UNIQUE INDEX ON UserTable (username) STORING (key);
CREATE UNIQUE INDEX ON UserTable (key) STORING (username);

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
    userid UUID PRIMARY KEY,
    -- Data columns
    cache  STRING
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
    CONSTRAINT unique_userid_hash
        UNIQUE (userid, hash)
);

CREATE
    INDEX ON Feed (userid)
    STORING (title, description, url, link, latest);

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
    -- Publication timestamp
    date      TIMESTAMPTZ,
    -- Retrieval timestamp
    retrieved TIMESTAMPTZ,
    CONSTRAINT unique_userid_feed_hash
        UNIQUE (userid, feed, hash)
);

CREATE
    UNIQUE INDEX IF NOT EXISTS article_idx_read_key
    ON Article (id) STORING (read);

CREATE
    INDEX ON Article (userid, id, read)
    STORING (title, summary, content, parsed, link, date);