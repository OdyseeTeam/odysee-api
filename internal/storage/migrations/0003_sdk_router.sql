-- +migrate Up

-- +migrate StatementBegin
CREATE TABLE lbrynet_servers (
    id SERIAL NOT NULL PRIMARY KEY,
    name VARCHAR NOT NULL,
    address VARCHAR NOT NULL,
    weight INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE (name)
);
-- +migrate StatementEnd

-- +migrate StatementBegin
ALTER TABLE users
    ADD COLUMN lbrynet_server_id INTEGER DEFAULT NULL REFERENCES lbrynet_servers(id) ON DELETE SET NULL;
-- +migrate StatementEnd

-- +migrate StatementBegin
-- This is for *local testing only*, you want to replace
-- these records with actual SDK addresses in a server environment.
INSERT INTO lbrynet_servers(name, address)
    VALUES
        ('default',  'http://localhost:15279/'),
        ('lbrynet2', 'http://localhost:15279/'),
        ('lbrynet3', 'http://localhost:15279/'),
        ('lbrynet4', 'http://localhost:15279/'),
        ('lbrynet5', 'http://localhost:15279/');
-- +migrate StatementEnd

-- +migrate StatementBegin
UPDATE users SET lbrynet_server_id = 1;
-- +migrate StatementEnd

-- +migrate Down

-- +migrate StatementBegin
ALTER TABLE users
    DROP COLUMN lbrynet_server_id;
-- +migrate StatementEnd

-- +migrate StatementBegin
DROP TABLE lbrynet_servers;
-- +migrate StatementEnd
