SET DATABASE TO Goliath;

-- Migrate Folder data

CREATE TABLE IF NOT EXISTS FolderWithId (
  -- Key columns
  userid UUID NOT NULL REFERENCES UserTable(id),
  id SERIAL NOT NULL UNIQUE,
  PRIMARY KEY (userid, id),
  -- Data columns
  name STRING UNIQUE
) INTERLEAVE IN PARENT UserTable (userid);

INSERT INTO FolderWithId (userid, id, name)
SELECT
    (SELECT id from UserTable LIMIT 1) AS userid,
    id, name FROM Folder;

-- Migrate FolderChildren data

CREATE TABLE IF NOT EXISTS FolderChildrenWithId (
  -- Key columns
  userid UUID NOT NULL REFERENCES UserTable(id),
  parent INT NOT NULL REFERENCES FolderWithId (id),
  child INT NOT NULL REFERENCES FolderWithId (id),
  PRIMARY KEY (userid, parent, child)
) INTERLEAVE IN PARENT FolderWithId (userid, parent);

INSERT INTO FolderChildrenWithId (userid, parent, child)
SELECT
    (SELECT id from UserTable LIMIT 1) AS userid,
    parent, child FROM FolderChildren;

-- Rename FolderChildren

DROP TABLE FolderChildren;

ALTER TABLE IF EXISTS FolderChildrenWithId RENAME TO FolderChildren;

-- Migrate Feed data

CREATE TABLE IF NOT EXISTS FeedWithId (
  -- Key columns
  userid UUID NOT NULL,
  folder INT NOT NULL,
  id SERIAL NOT NULL UNIQUE,
  PRIMARY KEY (userid, folder, id),
  CONSTRAINT fd_folder FOREIGN KEY (userid, folder) REFERENCES FolderWithId,
  -- Metadata columns
  hash STRING UNIQUE,
  -- Data columns
  title STRING,
  description STRING,
  url STRING,
  link STRING,
  -- MIME type of the favicon
  mime STRING,
  -- Base64 encoding of favicon
  favicon STRING,
  -- Latest timestamp of articles in this feed
  latest TIMESTAMPTZ DEFAULT CAST(0 AS TIMESTAMPTZ)
) INTERLEAVE IN PARENT FolderWithId (userid, folder);

INSERT INTO FeedWithId (userid, folder, id, hash, title, description, url, link, mime, favicon, latest)
SELECT
    (SELECT id from UserTable LIMIT 1) AS userid,
    folder, id, hash, title, description, url, link, mime, favicon, latest FROM Feed;

-- Migrate Article

CREATE TABLE IF NOT EXISTS ArticleWithId (
  -- Key columns
  userid UUID NOT NULL,
  folder INT NOT NULL,
  feed INT NOT NULL,
  id SERIAL NOT NULL UNIQUE,
  PRIMARY KEY (userid, folder, feed, id),
  CONSTRAINT fk_feed_folder FOREIGN KEY (userid, folder, feed) REFERENCES FeedWithId,
  -- Metadata columns
  hash STRING UNIQUE,
  -- Data columns
  title STRING,
  summary STRING,
  content STRING,
  parsed STRING,
  link STRING,
  read BOOL,
  -- Publication timestamp
  date TIMESTAMPTZ,
  -- Retrieval timestamp
  retrieved TIMESTAMPTZ
) INTERLEAVE IN PARENT FeedWithId (userid, folder, feed);

INSERT INTO ArticleWithId (userid, folder, feed, id, hash, title, summary, content, parsed, link, read, date, retrieved)
SELECT
    (SELECT id from UserTable LIMIT 1) AS userid,
    folder, feed, id, hash, title, summary, content, parsed, link, read, date, retrieved FROM Article;

-- Rename Article

DROP TABLE Article;

ALTER TABLE IF EXISTS ArticleWithId RENAME TO Article;

-- Rename Feed
-- NOTE: Feed can only be dropped after the old Article is dropped.

DROP TABLE Feed;

ALTER TABLE IF EXISTS FeedWithId RENAME TO Feed;

-- Rename Folder
-- NOTE: Folder can only be dropped after the old FolderChildren and Feed are
-- dropped.

DROP TABLE Folder;

ALTER TABLE IF EXISTS FolderWithId RENAME TO Folder;