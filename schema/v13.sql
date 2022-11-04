SET DATABASE TO Goliath;

ALTER TABLE IF EXISTS Feed
    DROP CONSTRAINT fd_folder;

ALTER TABLE IF EXISTS Feed
    ADD CONSTRAINT fk_folder_cascade FOREIGN KEY (userid, folder) REFERENCES folder
        ON UPDATE CASCADE;

ALTER TABLE IF EXISTS Article
    DROP CONSTRAINT fk_feed_folder;

ALTER TABLE IF EXISTS Article
    ADD CONSTRAINT fk_feed_folder_cascade FOREIGN KEY (userid, folder, feed) REFERENCES feed
        ON UPDATE CASCADE;