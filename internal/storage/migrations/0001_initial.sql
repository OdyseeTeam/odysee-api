-- +migrate Up

-- +migrate StatementBegin
CREATE DOMAIN uinteger AS integer
   CHECK(VALUE >= 0);
-- +migrate StatementEnd

-- +migrate StatementBegin
CREATE TABLE "users" (
    "id" uinteger NOT NULL PRIMARY KEY,

    "created_at" timestamp NOT NULL DEFAULT now(),
    "updated_at" timestamp NOT NULL DEFAULT now(),

    "sdk_account_id" varchar NOT NULL,
    "private_key" varchar NOT NULL,
    "public_key" varchar NOT NULL,
    "seed" varchar NOT NULL,

    UNIQUE ("sdk_account_id")
);
-- +migrate StatementEnd

-- +migrate Down

-- +migrate StatementBegin
DROP TABLE "users";
-- +migrate StatementEnd

-- +migrate StatementBegin
DROP DOMAIN uinteger;
-- +migrate StatementEnd
