-- name: UpsertSet :one
-- pack_image, mcm_id and release_date are COALESCEd so re-importing a set as
-- a *source* set (e.g. SPG pulled in by FDN's booster, with no sealed product
-- or release date of its own), or a re-import whose pack image fetch failed,
-- doesn't wipe what a previous primary import stored.
INSERT INTO sets (code, name, pack_image, mcm_id, release_date)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (code) DO UPDATE SET
    name         = EXCLUDED.name,
    pack_image   = COALESCE(EXCLUDED.pack_image, sets.pack_image),
    mcm_id       = COALESCE(EXCLUDED.mcm_id, sets.mcm_id),
    release_date = COALESCE(EXCLUDED.release_date, sets.release_date)
RETURNING *;

-- name: GetSetPackImage :one
SELECT pack_image FROM sets WHERE code = $1;

-- name: ListSetsWithMcmID :many
SELECT code, mcm_id FROM sets WHERE mcm_id IS NOT NULL;

-- name: UpdateSetPackPrices :exec
-- One statement for every set. A 0 price means "no figure" and is stored
-- as NULL (float8[] params can't carry NULLs through sqlc).
UPDATE sets s SET pack_price_eur = NULLIF(u.price, 0), pack_priced_at = @priced_at
FROM (SELECT unnest(@codes::text[]) AS code, unnest(@prices::float8[]) AS price) u
WHERE s.code = u.code;
