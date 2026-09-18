-- name: InsertPackOpen :one
INSERT INTO pack_opens (booster_config_id, pack_variant_index, seed)
VALUES ($1, $2, $3)
RETURNING *;

-- name: InsertPackOpenCard :exec
INSERT INTO pack_open_cards (pack_open_id, slot, sheet_name, foil, card_id)
VALUES ($1, $2, $3, $4, $5);

-- name: GetPackOpen :one
SELECT po.id, po.created_at, bc.set_code, bc.booster_type, bc.version AS config_version,
       s.pack_price_eur, s.pack_priced_at
FROM pack_opens po
JOIN booster_configs bc ON bc.id = po.booster_config_id
JOIN sets s ON s.code = bc.set_code
WHERE po.id = $1;

-- name: ListPackOpenCards :many
SELECT poc.slot, poc.sheet_name, poc.foil,
       c.id AS card_id, c.name, c.rarity, c.collector_number, c.set_code, c.image_uris,
       c.price_eur, c.price_foil_eur, c.priced_at
FROM pack_open_cards poc
JOIN cards c ON c.id = poc.card_id
WHERE poc.pack_open_id = $1
ORDER BY poc.slot;
