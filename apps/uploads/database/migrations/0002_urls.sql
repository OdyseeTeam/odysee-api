-- +migrate Up
-- +migrate StatementBegin
CREATE TYPE url_status AS ENUM (
    'created',
    'downloaded',
    'processed'
);

CREATE TABLE urls (
    id text NOT NULL UNIQUE PRIMARY KEY CHECK (id <> ''),
    user_id int NOT NULL,
    url text NOT NULL CHECK (url <> ''),
    filename text NOT NULL CHECK (filename <> ''),

    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp,

    status url_status NOT NULL,

    size bigint NOT NULL,
    sd_hash text NOT NULL,
    meta jsonb
);

-- CREATE INDEX uploads_id_user_id ON uploads(id, user_id);
-- CREATE INDEX uploads_id_user_id_status ON uploads(id, user_id, status);
-- +migrate StatementEnd

-- +migrate Down
-- +migrate StatementBegin
DROP TABLE urls;
DROP TYPE url_status;
-- +migrate StatementEnd
