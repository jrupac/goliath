-- Add titleoverridden column to Feed table, marking a title set by the user.

SET DATABASE TO Goliath;

-- False for a title that came from the feed, which is every title so far.
-- Fetching refreshes a feed's own metadata, so without this a renamed feed
-- reverts to whatever its XML says the next time the fetcher restarts.
ALTER TABLE Feed
    ADD COLUMN IF NOT EXISTS titleoverridden BOOL NOT NULL DEFAULT false;
