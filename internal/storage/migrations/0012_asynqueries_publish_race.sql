-- +migrate Up
-- +migrate StatementBegin
ALTER TABLE asynqueries ADD COLUMN ready_to_run boolean NOT NULL DEFAULT true;
ALTER TABLE asynqueries ADD COLUMN file_ready boolean NOT NULL DEFAULT false;
ALTER TABLE asynqueries ADD COLUMN file_meta jsonb;
-- +migrate StatementEnd

-- +migrate Down
-- +migrate StatementBegin
ALTER TABLE asynqueries DROP COLUMN ready_to_run;
ALTER TABLE asynqueries DROP COLUMN file_ready;
ALTER TABLE asynqueries DROP COLUMN file_meta;
-- +migrate StatementEnd
