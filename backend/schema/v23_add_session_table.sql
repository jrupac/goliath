-- Add Session table for revocable, expiring authentication sessions.

SET DATABASE TO Goliath;

CREATE TABLE IF NOT EXISTS Session
(
    -- Key columns
    id        UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    -- The bearer token is never stored; only its digest is, so that a leaked
    -- table (or database dump) does not hand over live sessions. Uniqueness is
    -- enforced by the covering index below rather than here, to avoid keeping
    -- two indexes on the same column.
    tokenhash BYTES       NOT NULL,
    userid    UUID        NOT NULL,
    -- Data columns
    scheme    STRING      NOT NULL,
    created   TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Expiry slides: a session is valid until it has gone unused for the
    -- configured idle window, so an actively syncing client never expires.
    lastseen  TIMESTAMPTZ NOT NULL DEFAULT now(),
    useragent STRING      NOT NULL DEFAULT '',
    CONSTRAINT session_userid_fkey
        FOREIGN KEY (userid)
            REFERENCES UserTable (id)
            ON DELETE CASCADE
);

-- The authentication path looks a session up by digest on every request and
-- reads only these columns, so serve it entirely from the index.
CREATE UNIQUE INDEX IF NOT EXISTS session_tokenhash_idx ON Session (tokenhash)
    STORING (userid, scheme, created, lastseen, useragent);

-- Listing a user's sessions, and revoking all of them at once.
CREATE INDEX IF NOT EXISTS session_userid_idx ON Session (userid);

-- Sweeping sessions that have gone idle past the expiry window.
CREATE INDEX IF NOT EXISTS session_lastseen_idx ON Session (lastseen);
