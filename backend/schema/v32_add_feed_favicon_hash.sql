-- Add a hash of each feed's favicon, and an index finding a favicon by it.
--
-- older-binaries: compatible
--
-- Older binaries never read the column, name every column they insert, and
-- cannot write it: it is computed from the favicon, so it stays right whoever
-- writes the favicon.
--
-- Favicons are served at a URL named by this hash, so that a client caches
-- each image once, under a name that changes when the image does. Feeds whose
-- favicons are byte for byte the same share a URL, whichever users they
-- belong to. The hash is of the image's bytes, not of their base64 encoding as
-- stored, so that it is the same as one computed from the image itself.

SET DATABASE TO Goliath;

ALTER TABLE Feed
    ADD COLUMN IF NOT EXISTS faviconhash STRING
        AS (sha256(decode(NULLIF(favicon, ''), 'base64'))) STORED;

-- Serving a favicon finds it by hash alone, across every user's feeds.
CREATE INDEX IF NOT EXISTS feed_faviconhash_idx
    ON Feed (faviconhash) WHERE faviconhash IS NOT NULL;

-- Listing a user's subscriptions names each feed's favicon by hash. A row
-- holds the favicon itself, which the table would otherwise read alongside the
-- hash; this holds only the hash.
CREATE INDEX IF NOT EXISTS feed_userid_faviconhash_idx
    ON Feed (userid) STORING (faviconhash)
    WHERE faviconhash IS NOT NULL AND deleted IS NULL;
