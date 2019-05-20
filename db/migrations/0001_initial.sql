-- +migrate Up

-- +migrate StatementBegin
CREATE TABLE "users" (
    "id" serial,
    "created" timestamp NOT NULL DEFAULT now(),
    "email" char(100) NOT NULL,
    "auth_token" char(32) NOT NULL,
    "is_identity_verified" bool NOT NULL DEFAULT false,
    "has_verified_email" bool NOT NULL DEFAULT false,
    "sdk_account_id" character varying(68) NOT NULL,
    "private_key" character varying(111) NOT NULL,
    "public_key" character varying(111) NOT NULL,
    "seed" character varying(1000) NOT NULL,
    PRIMARY KEY ("id"),
    UNIQUE ("auth_token"),
    UNIQUE ("sdk_account_id")
);
-- +migrate StatementEnd

-- +migrate Down
DROP TABLE users;
