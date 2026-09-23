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

-- name: UpdateCardPrices :exec
-- One statement for every card instead of a round-trip each. A 0 price
-- means "no figure" and is stored as NULL (float8[] params can't carry
-- NULLs through sqlc).
UPDATE cards c SET price_eur = NULLIF(u.price, 0), price_foil_eur = NULLIF(u.price_foil, 0), priced_at = @priced_at
FROM (SELECT unnest(@ids::uuid[]) AS id, unnest(@prices::float8[]) AS price, unnest(@foil_prices::float8[]) AS price_foil) u
WHERE c.id = u.id;
