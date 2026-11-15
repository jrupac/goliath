-- Add an index over saved articles.
--
-- older-binaries: compatible
--
-- Clients ask for the saved stream on every sync. With no index that
-- distinguishes saved articles, answering reads every article the user has,
-- bodies included, to find what is usually a handful.

SET DATABASE TO Goliath;

CREATE INDEX IF NOT EXISTS article_userid_saved_idx
    ON Article (userid, id) STORING (date) WHERE saved;
