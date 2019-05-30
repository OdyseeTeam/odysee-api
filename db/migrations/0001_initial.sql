-- +migrate Up

-- +migrate StatementBegin
CREATE TABLE "users" (
    "id" serial,
    "created_at" timestamp NOT NULL DEFAULT now(),
    "email" varchar NOT NULL,
    "auth_token" varchar NOT NULL,
    "is_identity_verified" bool NOT NULL DEFAULT false,
    "has_verified_email" bool NOT NULL DEFAULT false,
    "sdk_account_id" varchar NOT NULL,
    "private_key" varchar NOT NULL,
    "public_key" varchar NOT NULL,
    "seed" varchar NOT NULL,
    PRIMARY KEY ("id"),
    UNIQUE ("auth_token"),
    UNIQUE ("sdk_account_id")
);
-- +migrate StatementEnd

-- +migrate Down
DROP TABLE users;
