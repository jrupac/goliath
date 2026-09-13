-- Add deleted column to Feed table, marking a feed the user unsubscribed from.

SET DATABASE TO Goliath;

-- NULL for a live subscription. Unsubscribing sets it rather than deleting the
-- row, so that the request neither waits on removing every article the feed
-- holds nor races a fetch already in flight: every write the fetcher makes is
-- conditional on the feed being live. The garbage collector removes the feed
-- and its articles on its next run.
ALTER TABLE Feed
    ADD COLUMN IF NOT EXISTS deleted TIMESTAMPTZ;

-- Every read of a user's articles leaves out those of feeds they unsubscribed
-- from, by asking which of their feeds are tombstoned. Without this that
-- question reads every one of their feed rows, favicons included, on every
-- stream request; with it, it reads only the tombstones, which are usually
-- none.
CREATE
    INDEX IF NOT EXISTS feed_deleted_idx
    ON Feed (userid) WHERE deleted IS NOT NULL;
