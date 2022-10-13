-- +migrate Up
-- +migrate StatementBegin
ALTER TABLE publish_queries DROP CONSTRAINT publish_queries_pkey;
ALTER TABLE publish_queries
	ADD COLUMN id SERIAL NOT NULL PRIMARY KEY;
ALTER TABLE publish_queries
	ADD COLUMN user_id int REFERENCES users (id);
ALTER TABLE uploads
	ADD COLUMN query_id int REFERENCES publish_queries (id);

UPDATE
	uploads
SET
	query_id = q.id
FROM
	publish_queries q
WHERE
	q.upload_id = uploads.id;

UPDATE
	publish_queries
SET
	user_id = u.user_id
FROM
	uploads u
WHERE
	u.query_id = publish_queries.id;

ALTER TABLE publish_queries DROP COLUMN upload_id;

ALTER TABLE publish_queries RENAME TO queries;
ALTER TYPE publish_query_status RENAME TO query_status;
-- +migrate StatementEnd
