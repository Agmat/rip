-- +goose Up
-- Cardmarket product ids (MTGJSON's mcmId) and the latest Cardmarket
-- "trend" prices, refreshed by cmd/prices from Cardmarket's public daily
-- price guide. Prices are nullable: a card with no mcm_id, or one the
-- guide doesn't list, simply has no price.
ALTER TABLE cards
    ADD COLUMN mcm_id         int,
    ADD COLUMN price_eur      double precision,
    ADD COLUMN price_foil_eur double precision,
    ADD COLUMN priced_at      timestamptz;

ALTER TABLE sets
    ADD COLUMN mcm_id         int,
    ADD COLUMN pack_price_eur double precision,
    ADD COLUMN pack_priced_at timestamptz;

-- +goose Down
ALTER TABLE sets DROP COLUMN pack_priced_at, DROP COLUMN pack_price_eur, DROP COLUMN mcm_id;
ALTER TABLE cards DROP COLUMN priced_at, DROP COLUMN price_foil_eur, DROP COLUMN price_eur, DROP COLUMN mcm_id;
