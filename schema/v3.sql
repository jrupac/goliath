SET DATABASE TO Goliath;

CREATE UNIQUE INDEX IF NOT EXISTS article_idx_read_key
  ON Article (id) STORING (read);