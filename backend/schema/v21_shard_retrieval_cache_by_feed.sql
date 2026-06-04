-- Shard retrieval cache table by feed.

SET DATABASE TO Goliath;

DROP TABLE IF EXISTS RetrievalCache;

CREATE TABLE IF NOT EXISTS RetrievalCache
(
    userid UUID NOT NULL,
    feedid INT  NOT NULL,
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

GRANT ALL ON TABLE RetrievalCache to goliath;
