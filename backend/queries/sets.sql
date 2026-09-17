-- name: UpsertSet :one
INSERT INTO sets (code, name)
VALUES ($1, $2)
ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
RETURNING *;
