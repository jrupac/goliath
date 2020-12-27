SET
DATABASE TO Goliath;

CREATE TABLE IF NOT EXISTS RetrievalCache
(
    -- Key columns
    userid
    UUID
    PRIMARY
    KEY,
    -- Data columns
    cache
    STRING
) INTERLEAVE IN PARENT UserTable
(
    userid
);