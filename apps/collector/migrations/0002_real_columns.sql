-- +migrate Up

DROP TABLE events;


CREATE TYPE buffer_event_device AS ENUM('unknown', 'web', 'android');

-- +migrate StatementBegin
CREATE TABLE buffer_event (
  "id" SERIAL,
  "time" TIMESTAMP NOT NULL DEFAULT now(),
  "url" VARCHAR NOT NULL CHECK (url <> ''),
  "client" VARCHAR NOT NULL CHECK (client <> ''),
  "device" buffer_event_device NOT NULL,
  "position" INT NOT NULL,
  "duration" INT,
  "stream_duration" INT,
  "stream_bitrate" INT,
  "player" VARCHAR,
  "ready_state" SMALLINT NOT NULL,

  PRIMARY KEY(id, time)
);

CREATE INDEX client_idx on buffer_event(client);
CREATE INDEX device_idx ON buffer_event(device);
CREATE INDEX player_idx ON buffer_event(player);
CREATE INDEX ready_state_idx ON buffer_event(ready_state);
-- +migrate StatementEnd
