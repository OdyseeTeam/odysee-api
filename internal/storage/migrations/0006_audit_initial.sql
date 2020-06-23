-- +migrate Up

-- +migrate StatementBegin
CREATE TABLE "query_log" (
    "id" uinteger NOT NULL PRIMARY KEY,
    "method" varchar NOT NULL CHECK (method <> ''),
    "timestamp" timestamp NOT NULL DEFAULT now(),
    "user_id" uinteger,

    "remote_ip" varchar NOT NULL CHECK (remote_ip <> ''),
    "body" jsonb
);
CREATE INDEX queries_method_idx ON query_log(method);
CREATE INDEX queries_timestamp_idx ON query_log(timestamp);
CREATE INDEX queries_user_id_idx ON query_log(user_id);
CREATE INDEX queries_remote_ip_idx ON query_log(remote_ip);
-- +migrate StatementEnd

-- +migrate Down

-- +migrate StatementBegin
DROP TABLE "query_log";
-- +migrate StatementEnd
