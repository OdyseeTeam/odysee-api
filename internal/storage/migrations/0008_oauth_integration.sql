-- +migrate Up

-- +migrate StatementBegin
ALTER TABLE users ADD COLUMN idp_id varchar DEFAULT NULL;
-- +migrate StatementEnd

-- +migrate StatementBegin
CREATE INDEX users_idp_id_idx ON users(idp_id);
-- +migrate StatementEnd

-- +migrate Down

-- +migrate StatementBegin
ALTER TABLE users DROP COLUMN  idp_id;
-- +migrate StatementEnd
