-- name: UpsertCard :one
INSERT INTO cards (id, set_code, name, rarity, collector_number, scryfall_id, image_uris, finishes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET
    name             = EXCLUDED.name,
    rarity           = EXCLUDED.rarity,
    collector_number = EXCLUDED.collector_number,
    scryfall_id      = EXCLUDED.scryfall_id,
    image_uris       = EXCLUDED.image_uris,
    finishes         = EXCLUDED.finishes
RETURNING *;
