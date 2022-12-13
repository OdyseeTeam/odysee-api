-- +migrate Up
-- +migrate StatementBegin
CREATE TYPE  asynquery_status AS ENUM (
    'received',
    'forwarded',
    'failed',
    'succeeded'
);

CREATE TYPE forklift_upload_status AS ENUM (
    'created',
    'uploading',
    'received',
    'terminated',
    'abandoned',
    'failed',
    'finished'
);

CREATE TABLE asynqueries (
    id SERIAL NOT NULL PRIMARY KEY,
    user_id int REFERENCES users (id),
    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp,

    status asynquery_status NOT NULL,
    error text NOT NULL,

    query jsonb,
    response jsonb
);

CREATE TABLE forklift_uploads (
    id text NOT NULL UNIQUE PRIMARY KEY CHECK (id <> ''),
    user_id int REFERENCES users(id) ON DELETE SET NULL,
    asynquery_id int REFERENCES asynqueries (id),
    path text NOT NULL,

    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp,

    status forklift_upload_status NOT NULL,
    error text NOT NULL,

    size bigint NOT NULL CHECK (size > 0),
    received bigint NOT NULL DEFAULT 0
);

CREATE INDEX asynqueries_id_user_id ON asynqueries(id, user_id);
CREATE INDEX forklift_uploads_id_user_id ON asynqueries(id, user_id);
-- +migrate StatementEnd

-- +migrate Down
-- +migrate StatementBegin
DROP TABLE forklift_uploads;
DROP TABLE asynqueries;
DROP TYPE forklift_upload_status;
DROP TYPE asynquery_status;
-- +migrate StatementEnd
