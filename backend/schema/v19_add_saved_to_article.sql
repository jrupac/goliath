-- Add "saved" column to Article table with a default value of false.

SET
DATABASE TO Goliath;

ALTER TABLE Article
    ADD COLUMN saved BOOL DEFAULT false;
