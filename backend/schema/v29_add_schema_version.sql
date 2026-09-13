-- Add SchemaVersion table, recording which migrations a database has had.
--
-- older-binaries: compatible

SET DATABASE TO Goliath;

-- One row per migration, written by goliath-cli once the migration has
-- applied, and read by the application at startup to decide whether it can run
-- on this database. The migrations do not write it themselves: the version is
-- already in each file's name.
CREATE TABLE IF NOT EXISTS SchemaVersion
(
    version               INT PRIMARY KEY,
    name                  STRING      NOT NULL,
    applied               TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Whether a binary built before this version must refuse to run once it
    -- has been applied
    breaks_older_binaries BOOL        NOT NULL
);

-- The application only reads it; goliath-cli writes it as root.
GRANT SELECT ON TABLE SchemaVersion TO goliath;
