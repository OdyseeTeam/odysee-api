-- +migrate Up

-- +migrate StatementBegin
ALTER TABLE "users"
    DROP COLUMN "private_key",
    DROP COLUMN "public_key",
    DROP COLUMN "seed",

    ADD COLUMN "wallet_id" varchar NOT NULL DEFAULT '',

    ALTER COLUMN "sdk_account_id" DROP NOT NULL
;
-- +migrate StatementEnd

-- +migrate Down

-- +migrate StatementBegin
ALTER TABLE "users"
    ADD COLUMN "private_key" varchar NOT NULL DEFAULT '',
    ADD COLUMN "public_key" varchar NOT NULL DEFAULT '',
    ADD COLUMN "seed" varchar NOT NULL DEFAULT '',

    DROP COLUMN "wallet_id"
;
-- +migrate StatementEnd
