-- name: UpsertSet :one
INSERT INTO sets (code, name, pack_image_url)
VALUES ($1, $2, $3)
ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, pack_image_url = EXCLUDED.pack_image_url
RETURNING *;
