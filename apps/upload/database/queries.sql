-- name: CreateUpload :one
INSERT INTO uploads (
    id, user_id, size, status, filename, key
) VALUES (
  $1, $2, $3, 'created', '', ''
)
RETURNING *;

-- name: GetUpload :one
SELECT * FROM uploads
WHERE user_id = $1 AND id = $2;

-- name: RecordUploadProgress :exec
UPDATE uploads SET
    updated_at = NOW(),
    received = $3
WHERE user_id = $1 AND id = $2 AND status IN ('receiving', 'created');

-- name: MarkUploadTerminated :exec
UPDATE uploads SET
    updated_at = NOW(),
    status = 'completed'
WHERE user_id = $1 AND id = $2;

-- name: MarkUploadCompleted :exec
UPDATE uploads SET
    updated_at = NOW(),
    status = 'completed',
    filename = $3,
    key = $4
WHERE user_id = $1 AND id = $2;
