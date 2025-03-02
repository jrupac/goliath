-- Add UserUnmuteFeeds table storing user ids and feed ids which should never
-- be muted for a user even if a mute word matches an article from it.
-- Also update the foreign key constraint on UserPrefs to delete the row if
-- the user is deleted.

SET DATABASE TO Goliath;

CREATE TABLE IF NOT EXISTS UserUnmuteFeeds
(
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

-- Ensure that the goliath user can operate on the new table
GRANT ALL ON TABLE UserUnmuteFeeds to goliath;

-- Remove constraint if it exists first
ALTER TABLE UserPrefs
    DROP CONSTRAINT IF EXISTS userprefs_userid_fkey;

ALTER TABLE UserPrefs
    ADD CONSTRAINT userprefs_userid_fkey
        FOREIGN KEY (userid)
            REFERENCES UserTable (id)
            ON DELETE CASCADE;