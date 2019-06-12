-- +migrate Up

-- +migrate StatementBegin
CREATE TABLE "users" (
    "id" integer NOT NULL,
    "created_at" timestamp NOT NULL DEFAULT now(),
    "updated_at" timestamp NOT NULL DEFAULT now(),
    "sdk_account_id" varchar NOT NULL,
    "private_key" varchar NOT NULL,
    "public_key" varchar NOT NULL,
    "seed" varchar NOT NULL,
    PRIMARY KEY ("id"),
    UNIQUE ("sdk_account_id")
);
-- +migrate StatementEnd

-- +migrate Down
DROP TABLE users;
