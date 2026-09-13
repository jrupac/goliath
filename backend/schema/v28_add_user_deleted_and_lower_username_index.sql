-- Let a user be deleted and restored before being purged, and make usernames
-- unique regardless of case.

SET DATABASE TO Goliath;

-- NULL for a live user. Deleting a user sets it, revokes their sessions and
-- stops fetching their feeds, but leaves what they own in place: removing
-- everything at once can mean hundreds of thousands of rows held under lock in
-- one transaction, and leaving it lets the deletion be undone. The garbage
-- collector purges the user once the retention window has passed.
ALTER TABLE UserTable
    ADD COLUMN IF NOT EXISTS deleted TIMESTAMPTZ;

-- Two names differing only in case would be indistinguishable to the people
-- typing them. Adding a user checks for such a name before inserting, and this
-- makes that check a point lookup rather than a scan of every username, which
-- concurrent adds would otherwise all contend on. It also enforces the rule for
-- users inserted by any other path. A deleted user's name stays taken until
-- they are purged, so that restoring them cannot collide.
CREATE UNIQUE INDEX IF NOT EXISTS usertable_username_lower_key
    ON UserTable (lower(username));
