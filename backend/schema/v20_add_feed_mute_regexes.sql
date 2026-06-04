-- Add UserFeedMuteRegexes table for per-feed regex mute rules.

SET DATABASE TO Goliath;

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

GRANT ALL ON TABLE UserFeedMuteRegexes to goliath;

