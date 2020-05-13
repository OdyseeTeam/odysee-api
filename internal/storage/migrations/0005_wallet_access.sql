-- +migrate Up

ALTER TABLE users ADD COLUMN "last_seen_at" timestamp DEFAULT NULL;
CREATE INDEX users_last_seen_at_idx ON users(last_seen_at);


-- +migrate Down

ALTER TABLE users DROP COLUMN "last_seen_at";
