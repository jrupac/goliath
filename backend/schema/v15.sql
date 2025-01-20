SET DATABASE TO Goliath;

CREATE TABLE IF NOT EXISTS UserPrefs
(
    userid     UUID NOT NULL REFERENCES UserTable (id) PRIMARY KEY,
    mute_words STRING[]
);
