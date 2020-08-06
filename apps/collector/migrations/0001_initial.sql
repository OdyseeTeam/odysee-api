-- +migrate Up

-- +migrate StatementBegin
CREATE TABLE "events" (
    "id" SERIAL,
    "time" TIMESTAMP NOT NULL DEFAULT now(),
    "type" VARCHAR NOT NULL CHECK (type <> ''),
    "client" VARCHAR NOT NULL CHECK (client <> ''),
    "device" VARCHAR,
    "data" JSONB,

    PRIMARY KEY(id, time)
);

CREATE INDEX type_idx ON events(type);
-- +migrate StatementEnd

-- +migrate Down

-- +migrate StatementBegin
DROP TABLE "events";
-- +migrate StatementEnd
