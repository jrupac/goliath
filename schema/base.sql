CREATE DATABASE IF NOT EXISTS Goliath;

-- User "goliath" must already exist.
GRANT ALL ON DATABASE Goliath TO goliath;

SET DATABASE TO Goliath;

CREATE TABLE IF NOT EXISTS UserTable (
  -- Key columns
  username STRING PRIMARY KEY,
  -- Data columns
  key STRING NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS Folder (
  -- Key columns
  id SERIAL PRIMARY KEY,
  -- Data columns
  name STRING
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
  -- MIME type of the favicon
  mime STRING,
  -- Base64 encoding of favicon
  favicon STRING
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
  link STRING,
  date TIMESTAMPTZ,
  read BOOL
) INTERLEAVE IN PARENT Feed (folder, feed);