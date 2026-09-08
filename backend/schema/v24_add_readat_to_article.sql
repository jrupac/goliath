-- Add readat column to Article table, recording when an article was marked read.

SET DATABASE TO Goliath;

-- NULL for an unread article, and for an article that was already read when
-- this column was added. Clients reconcile read state by asking what became
-- read since a given time, which the read flag alone cannot answer.
ALTER TABLE Article
    ADD COLUMN IF NOT EXISTS readat TIMESTAMPTZ;

-- Answering "what became read since <time>" for one user. The existing
-- (userid, id, read) index does not store readat, so without this the read
-- stream joins back to the primary index once per read article.
CREATE
    INDEX IF NOT EXISTS article_idx_readat
    ON Article (userid, readat) STORING (read, date);
