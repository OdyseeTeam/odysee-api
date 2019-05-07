-- +migrate Up

-- +migrate StatementBegin
CREATE TABLE "users" (
    "id" serial,
    "created" timestamp NOT NULL DEFAULT now(),
    "auth_token" char(32),
    "is_identity_verified" bool,
    "has_verified_email" bool,
    "sdk_account_id" character varying(68),
    "private_key" character varying(111),
    "public_key" character varying(111),
    "seed" character varying(1000),
    PRIMARY KEY ("id"),
    UNIQUE ("auth_token"),
    UNIQUE ("sdk_account_id")
);
-- +migrate StatementEnd

-- +migrate Down
DROP TABLE users;
