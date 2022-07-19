-- +migrate Up

CREATE TYPE upload_status AS ENUM (
    'created',
    'uploading',
    'uploaded',
    'query_sent',
    'query_failed',
    'completed',
    'terminated',
    'abandoned'
);

CREATE TABLE uploads (
    id text NOT NULL UNIQUE PRIMARY KEY CHECK (id <> ''),
    user_id int REFERENCES users(id) ON DELETE SET NULL,

    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp,

    status upload_status NOT NULL,

    size bigint NOT NULL CHECK (size > 0),
    received bigint NOT NULL DEFAULT 0
);

CREATE TABLE upload_queries (
    upload_id text NOT NULL PRIMARY KEY references uploads(id),

    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp,

    query jsonb,
    response jsonb,
    error text NOT NULL
);

-- +migrate Down
DROP TABLE upload_queries;
DROP TABLE uploads;
DROP TYPE upload_status;
