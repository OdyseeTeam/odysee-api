-- +migrate Up

-- +migrate StatementBegin
ALTER TABLE lbrynet_servers
    ADD COLUMN "private" BOOLEAN NOT NULL DEFAULT false;
-- +migrate StatementEnd

-- +migrate Down

-- +migrate StatementBegin
ALTER TABLE lbrynet_servers
    DROP COLUMN "private";
-- +migrate StatementEnd
