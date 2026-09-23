-- name: NextBoosterConfigVersion :one
SELECT COALESCE(MAX(version), 0) + 1 AS next_version
FROM booster_configs
WHERE set_code = $1 AND booster_type = $2;

-- name: DeactivateBoosterConfigs :exec
UPDATE booster_configs SET is_active = false
WHERE set_code = $1 AND booster_type = $2 AND is_active;

-- name: InsertBoosterConfig :one
INSERT INTO booster_configs (set_code, booster_type, version, config, is_active)
VALUES ($1, $2, $3, $4, true)
RETURNING *;

-- name: GetActiveBoosterConfig :one
SELECT * FROM booster_configs
WHERE set_code = $1 AND booster_type = $2 AND is_active;

-- name: ListActiveBoosterConfigsWithSetName :many
-- pack_image_hash is NULL for a set with no image; COALESCE to "" (treated
-- as "no pack image" in Go) so sqlc keeps the column a plain string.
SELECT bc.set_code, s.name AS set_name, COALESCE(s.pack_image_hash, '')::text AS pack_image_hash, bc.booster_type,
       s.pack_price_eur, s.pack_priced_at
FROM booster_configs bc
JOIN sets s ON s.code = bc.set_code
WHERE bc.is_active
ORDER BY s.release_date DESC NULLS LAST, bc.set_code, bc.booster_type;
