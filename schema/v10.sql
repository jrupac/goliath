SET DATABASE TO Goliath;

CREATE UNIQUE INDEX ON UserTable (username) STORING (key);
CREATE UNIQUE INDEX ON UserTable (key) STORING (username);

CREATE INDEX ON Feed (userid) STORING (title, description, url, link, latest);

CREATE INDEX ON Article (userid, id, read)
    STORING (title, summary, content, parsed, link, date);