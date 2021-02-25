-- +migrate Up

-- +migrate StatementBegin
CREATE INDEX url_idx ON buffer_event(url);
CREATE INDEX time_idx ON buffer_event(time);
-- +migrate StatementEnd

-- +migrate Down
-- +migrate StatementBegin
DROP INDEX url_idx;
DROP INDEX time_idx;
-- +migrate StatementEnd
