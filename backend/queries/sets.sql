-- name: UpsertSet :one
INSERT INTO sets (code, name, pack_image)
VALUES ($1, $2, $3)
ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, pack_image = EXCLUDED.pack_image
RETURNING *;

-- name: GetSetPackImage :one
SELECT pack_image FROM sets WHERE code = $1;
