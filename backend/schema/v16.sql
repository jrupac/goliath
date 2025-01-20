SET DATABASE TO Goliath;

-- Fix some uniqueness constraints. Previously, folder names, feed hashes, and
-- article hashes were enforced unique across their entire tables. This would
-- not work with multiple users. Instead, drop that column constraint and add
-- new constraints that include the userid. In the case of the article one,
-- also keep the feed id in the constraint (since the same article hash could
-- appear in multiple feeds).

DROP INDEX Folder@folderwithid_name_key CASCADE;

ALTER TABLE Folder
    ADD CONSTRAINT unique_userid_feed_hash UNIQUE (userid, name);

DROP INDEX Feed@feedwithid_hash_key CASCADE;

ALTER TABLE Feed
    ADD CONSTRAINT unique_userid_feed_hash UNIQUE (userid, hash);

DROP INDEX Article@articlewithid_hash_key CASCADE;

ALTER TABLE Article
    ADD CONSTRAINT unique_userid_feed_hash UNIQUE (userid, feed, hash);