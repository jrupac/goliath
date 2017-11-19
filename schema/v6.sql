SET DATABASE TO Goliath;

ALTER TABLE IF EXISTS Article ADD COLUMN parsed STRING;

UPDATE Article set parsed = '';