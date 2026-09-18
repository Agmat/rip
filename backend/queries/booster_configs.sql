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
SELECT bc.set_code, s.name AS set_name, s.pack_image_url, bc.booster_type
FROM booster_configs bc
JOIN sets s ON s.code = bc.set_code
WHERE bc.is_active
ORDER BY bc.set_code, bc.booster_type;
