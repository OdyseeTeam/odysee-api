-- +migrate Up
-- +migrate StatementBegin
CREATE TABLE sdk_reconnect_leases (
    lease_key text PRIMARY KEY,
    lease_kind text NOT NULL CHECK (lease_kind IN ('endpoint', 'global_slot')),
    owner text,
    lease_expires_at timestamptz,
    last_reconnect_at timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts_alerted_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);
-- +migrate StatementEnd

-- +migrate Down
-- +migrate StatementBegin
DROP TABLE sdk_reconnect_leases;
-- +migrate StatementEnd
