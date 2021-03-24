-- +migrate Up

-- +migrate StatementBegin
CREATE TABLE "playback_reports" (
    "id" SERIAL,
    "time" TIMESTAMP NOT NULL DEFAULT now(),

    "url" VARCHAR NOT NULL CHECK (url <> ''),
    "pos" INT NOT NULL,
    "por" INT NOT NULL,
    "dur" INT NOT NULL,
    "bfc" INT NOT NULL,
    "bfd" INT NOT NULL,
    "fmt" VARCHAR NOT NULL CHECK (fmt <> ''),
    "pid" VARCHAR NOT NULL CHECK (pid <> ''),
    "cid" VARCHAR NOT NULL CHECK (cid <> ''),
    "cdv" VARCHAR NOT NULL CHECK (cdv <> ''),
    "crt" INT NOT NULL DEFAULT 0,
    "car" VARCHAR NOT NULL DEFAULT '',

    PRIMARY KEY(id)
);

CREATE INDEX time_idx ON playback_reports(time);
CREATE INDEX url_idx ON playback_reports(url);
CREATE INDEX fmt_idx ON playback_reports(fmt);
CREATE INDEX pid_idx ON playback_reports(pid);
CREATE INDEX cid_idx ON playback_reports(cid);
-- +migrate StatementEnd

-- +migrate Down

-- +migrate StatementBegin
DROP TABLE "playback_reports";
-- +migrate StatementEnd
