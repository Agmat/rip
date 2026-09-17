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
