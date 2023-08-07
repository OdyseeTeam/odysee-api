-- +migrate Up
-- +migrate StatementBegin
ALTER TABLE asynqueries
  DROP CONSTRAINT asynqueries_upload_id_key;
-- +migrate StatementEnd

-- +migrate Down
-- +migrate StatementBegin
ALTER TABLE asynqueries
  ADD CONSTRAINT asynqueries_upload_id_key UNIQUE (upload_id);
-- +migrate StatementEnd
