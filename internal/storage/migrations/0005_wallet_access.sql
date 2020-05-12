-- +migrate Up

ALTER TABLE users ADD COLUMN "last_seen_at" timestamp DEFAULT NULL;


-- +migrate Down

ALTER TABLE users DROP COLUMN "last_seen_at";
