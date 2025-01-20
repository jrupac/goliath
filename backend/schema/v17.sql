SET DATABASE TO Goliath;

-- Ensure that the goliath user can operate on the new table
GRANT ALL ON TABLE UserPrefs to goliath;

-- Initialize the UserPrefs table with an empty array of mute words for all
-- users.
INSERT INTO UserPrefs (userid, mute_words)
SELECT id, ARRAY []::STRING[]
FROM UserTable
ON CONFLICT (userid) DO NOTHING;