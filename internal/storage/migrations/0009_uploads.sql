-- +migrate Up

CREATE TYPE upload_status AS ENUM (
    'created',
    'uploading',
    'received',
    -- 'analyzed',
    -- 'split',
    -- 'reflected',
    -- 'posted', -- query sent
    'terminated',
    'abandoned',
    'failed',
    'finished'
);

CREATE TYPE publish_query_status AS ENUM (
    'received',
    'forwarded',
    'failed',
    'succeeded'
);

CREATE TABLE uploads (
    id text NOT NULL UNIQUE PRIMARY KEY CHECK (id <> ''),
    user_id int REFERENCES users(id) ON DELETE SET NULL,
    path text NOT NULL,

    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp,

    status upload_status NOT NULL,
    error text NOT NULL,

    size bigint NOT NULL CHECK (size > 0),
    received bigint NOT NULL DEFAULT 0
);

CREATE TABLE publish_queries (
    upload_id text NOT NULL PRIMARY KEY references uploads(id),

    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp,

    status publish_query_status NOT NULL,
    error text NOT NULL,

    query jsonb,
    response jsonb
);

-- +migrate Down
DROP TABLE publish_queries;
DROP TABLE uploads;
DROP TYPE upload_status;
DROP TYPE publish_query_status;
