-- Re-key Feed and Article without the folder, and drop Article's folder column.
--
-- A feed's folder was part of the primary key of both tables, so moving a feed
-- rewrote every article it had, row and index entries alike, through an
-- ON UPDATE CASCADE foreign key. Here it becomes an ordinary column of Feed,
-- and an article's folder is its feed's.
--
-- older-binaries: incompatible
--
-- Older binaries write and read Article.folder, and rely on the cascade to
-- keep it right.
--
-- Every statement is conditional, so that a run stopped part-way can be run
-- again. The primary keys are not named: databases differ in what their
-- constraints are called, and ALTER PRIMARY KEY keeps whichever name a key
-- already has. What it leaves behind is named after columns instead, the same
-- everywhere.

SET DATABASE TO Goliath;

-- Article's foreign key names Feed's current key, which cannot change while
-- it is referenced.
ALTER TABLE Article DROP CONSTRAINT IF EXISTS fk_feed_folder_cascade;

-- Changing a primary key keeps the old one as a unique index; nothing needs
-- it. Changing a key to the columns it already has does nothing, which is
-- what a second run does.
ALTER TABLE Feed ALTER PRIMARY KEY USING COLUMNS (userid, id);
DROP INDEX IF EXISTS Feed@feed_userid_folder_id_key;

ALTER TABLE Article ALTER PRIMARY KEY USING COLUMNS (userid, feed, id);
DROP INDEX IF EXISTS Article@article_userid_folder_feed_id_key;

-- Not in the same statement as the key change, which refuses to drop a column
-- the key it is replacing still names.
ALTER TABLE Article DROP COLUMN IF EXISTS folder;

ALTER TABLE Article
    ADD CONSTRAINT IF NOT EXISTS fk_feed
        FOREIGN KEY (userid, feed) REFERENCES Feed (userid, id);

-- A folder's feeds, which reading or marking a folder's articles asks for, and
-- which the key no longer serves.
CREATE INDEX IF NOT EXISTS feed_userid_folder_idx ON Feed (userid, folder);
