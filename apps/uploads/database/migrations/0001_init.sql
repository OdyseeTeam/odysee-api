-- +migrate Up
-- +migrate StatementBegin
CREATE TYPE upload_status AS ENUM (
    'created',
    'receiving',
    'completed',
    'terminated',
    'processed'
);

CREATE TABLE uploads (
    id text NOT NULL UNIQUE PRIMARY KEY CHECK (id <> ''),
    user_id int NOT NULL,
    filename text NOT NULL,
    key text NOT NULL,

    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp,

    status upload_status NOT NULL,

    size bigint NOT NULL CHECK (size > 0),
    received bigint NOT NULL DEFAULT 0,

    sd_hash text NOT NULL,
    meta jsonb
);

CREATE INDEX uploads_id_user_id ON uploads(id, user_id);
CREATE INDEX uploads_id_user_id_status ON uploads(id, user_id, status);
-- +migrate StatementEnd

-- +migrate Down
-- +migrate StatementBegin
DROP TABLE uploads;
DROP TYPE upload_status;
-- +migrate StatementEnd
