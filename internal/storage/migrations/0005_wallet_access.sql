-- +migrate Up

ALTER TABLE users ADD COLUMN "wallet_accessed_at" timestamp DEFAULT NULL;


-- +migrate Down

ALTER TABLE users DROP COLUMN "wallet_accessed_at";
