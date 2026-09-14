-- Add an index looking a user's feed up by URL.
--
-- older-binaries: compatible

SET DATABASE TO Goliath;

-- Adding a feed first asks whether the user already has it, or had it and
-- unsubscribed. Without this, that lookup reads every feed the user has, and
-- so waits on any of them being written: a fetch recording its latest time, or
-- a move rewriting a feed's key.
CREATE INDEX IF NOT EXISTS feed_userid_url_idx ON Feed (userid, url);
