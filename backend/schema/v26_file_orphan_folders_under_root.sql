-- File every folder that has no parent under the root folder.

SET DATABASE TO Goliath;

-- Every folder but the root belongs under a parent. One without still holds
-- feeds and is still listed to clients, but anything walking the hierarchy from
-- the root, such as OPML export, passes over it and every feed in it.
INSERT INTO FolderChildren (userid, parent, child)
SELECT f.userid, root.id, f.id
FROM Folder f
JOIN Folder root ON root.userid = f.userid AND root.name = '<root>'
WHERE f.id != root.id
  AND NOT EXISTS (
    SELECT 1 FROM FolderChildren c WHERE c.userid = f.userid AND c.child = f.id
  )
ON CONFLICT (userid, parent, child) DO NOTHING;
