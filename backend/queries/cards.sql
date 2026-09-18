-- name: UpsertCard :one
-- Price columns are deliberately not touched here: re-importing a set
-- refreshes card data, not prices (cmd/prices owns those).
INSERT INTO cards (id, set_code, name, rarity, collector_number, scryfall_id, image_uris, finishes, mcm_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO UPDATE SET
    name             = EXCLUDED.name,
    rarity           = EXCLUDED.rarity,
    collector_number = EXCLUDED.collector_number,
    scryfall_id      = EXCLUDED.scryfall_id,
    image_uris       = EXCLUDED.image_uris,
    finishes         = EXCLUDED.finishes,
    mcm_id           = EXCLUDED.mcm_id
RETURNING *;

-- name: ListCardsWithMcmID :many
SELECT id, mcm_id FROM cards WHERE mcm_id IS NOT NULL;

-- name: UpdateCardPrice :exec
UPDATE cards SET price_eur = $2, price_foil_eur = $3, priced_at = $4 WHERE id = $1;
