CREATE DATABASE IF NOT EXISTS Goliath;

-- User "goliath" must already exist.
GRANT ALL ON DATABASE Goliath TO goliath;

SET DATABASE TO Goliath;

CREATE TABLE IF NOT EXISTS UserTable (
  -- Key columns
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username STRING NOT NULL UNIQUE,
  -- Data columns
  key STRING NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS Folder (
  -- Key columns
  id SERIAL PRIMARY KEY,
  -- Data columns
  name STRING UNIQUE
);

CREATE TABLE IF NOT EXISTS FolderChildren (
  -- Key columns
  parent INT NOT NULL REFERENCES Folder (id),
  child INT NOT NULL REFERENCES Folder (id),
  PRIMARY KEY (parent, child)
) INTERLEAVE IN PARENT Folder (parent);

CREATE TABLE IF NOT EXISTS Feed (
  -- Key columns
  id SERIAL NOT NULL UNIQUE,
  folder INT NOT NULL,
  PRIMARY KEY (folder, id),
  CONSTRAINT fd_folder FOREIGN KEY (folder) REFERENCES Folder,
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
) INTERLEAVE IN PARENT Folder (folder);

CREATE TABLE IF NOT EXISTS Article (
  -- Key columns
  id SERIAL NOT NULL UNIQUE,
  feed INT NOT NULL,
  folder INT NOT NULL,
  PRIMARY KEY (folder, feed, id),
  CONSTRAINT fk_feed_folder FOREIGN KEY (folder, feed) REFERENCES Feed,
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
) INTERLEAVE IN PARENT Feed (folder, feed);

CREATE UNIQUE INDEX IF NOT EXISTS article_idx_read_key
  ON Article (id) STORING (read);