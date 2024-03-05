SET DATABASE TO Goliath;

ALTER TABLE RetrievalCache
    ALTER PRIMARY KEY USING COLUMNS (userid);

ALTER TABLE Article
    ALTER PRIMARY KEY USING COLUMNS (userid, folder, feed, id);

ALTER TABLE Feed
    ALTER PRIMARY KEY USING COLUMNS (userid, folder, id);

ALTER TABLE FolderChildren
    ALTER PRIMARY KEY USING COLUMNS (userid, parent, child);

ALTER TABLE Folder
    ALTER PRIMARY KEY USING COLUMNS (userid, id);