-- +goose Up
CREATE TABLE sets (
    code       text PRIMARY KEY,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE cards (
    id             uuid PRIMARY KEY, -- MTGJSON card uuid; also the id referenced by booster sheet weights
    set_code       text NOT NULL REFERENCES sets (code),
    name           text NOT NULL,
    rarity         text NOT NULL CHECK (rarity IN ('common', 'uncommon', 'rare', 'mythic', 'special', 'bonus')),
    collector_number text NOT NULL,
    scryfall_id    uuid NOT NULL,
    image_uris     jsonb NOT NULL, -- Scryfall image_uris, or {faces: [...]} for double-faced cards
    finishes       text[] NOT NULL, -- e.g. {nonfoil,foil}
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX cards_set_code_idx ON cards (set_code);

-- One row per imported (set, booster type) pack configuration. `config` holds
-- MTGJSON's booster block close to verbatim (boosters[] variants + sheets{}),
-- so the draw engine and a replay both work off the exact structure that was
-- live at open time. Re-importing a set creates a new version rather than
-- mutating an existing one.
CREATE TABLE booster_configs (
    id           bigserial PRIMARY KEY,
    set_code     text NOT NULL REFERENCES sets (code),
    booster_type text NOT NULL, -- e.g. 'play', 'collector'
    version      int NOT NULL,
    config       jsonb NOT NULL,
    is_active    boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (set_code, booster_type, version)
);

-- Only one active config per (set, booster_type) at a time.
CREATE UNIQUE INDEX booster_configs_active_idx ON booster_configs (set_code, booster_type) WHERE is_active;

-- +goose Down
DROP TABLE booster_configs;
DROP TABLE cards;
DROP TABLE sets;
