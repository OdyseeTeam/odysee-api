-- +migrate Up
-- +migrate StatementBegin
CREATE TYPE  asynquery_status AS ENUM (
    'received',
    'preparing',
    'forwarded',
    'failed',
    'succeeded'
);

CREATE TABLE asynqueries (
    id text NOT NULL UNIQUE PRIMARY KEY CHECK (id <> ''),
    user_id int REFERENCES users (id) NOT NULL,
    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp,

    status asynquery_status NOT NULL,
    error text NOT NULL,
    upload_id text NOT NULL UNIQUE,

    body jsonb,
    response jsonb
);

CREATE INDEX asynqueries_id_user_id ON asynqueries(id, user_id);
CREATE INDEX asynqueries_user_id_upload_id ON asynqueries(user_id, upload_id);
-- +migrate StatementEnd

-- +migrate Down
-- +migrate StatementBegin
DROP TABLE asynqueries;
DROP TYPE asynquery_status;
-- +migrate StatementEnd
