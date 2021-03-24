-- name: CreatePlaybackReport :exec

INSERT INTO playback_reports (url, pos, por, dur, bfc, bfd, fmt, pid, cid, cdv, crt, car) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
);
