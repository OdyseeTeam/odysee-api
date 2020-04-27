-- +migrate Up

UPDATE users SET lbrynet_server_id = (id % (SELECT COUNT(id) from lbrynet_servers))+1 WHERE lbrynet_server_id IS NULL;
ALTER TABLE users DROP COLUMN wallet_id;

-- +migrate Down

ALTER TABLE "users" ADD COLUMN "wallet_id" varchar NOT NULL DEFAULT '';
