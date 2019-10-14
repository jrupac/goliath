SET DATABASE TO Goliath;

CREATE TABLE IF NOT EXISTS UserTableWithId (
  -- Key columns
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username STRING NOT NULL UNIQUE,
  -- Data columns
  key STRING NOT NULL UNIQUE
);

INSERT INTO UserTableWithId (username, key) SELECT username, key FROM UserTable;

DROP TABLE UserTable;

ALTER TABLE IF EXISTS UserTableWithId RENAME TO UserTable;