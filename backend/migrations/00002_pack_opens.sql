-- +goose Up

-- One row per pack opened. seed + booster_config_id + pack_variant_index
-- fully determine the draw: re-running the same draw algorithm against them
-- reproduces the exact same cards, which is what makes an open replayable
-- and auditable without trusting client input.
CREATE TABLE pack_opens (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    booster_config_id  bigint NOT NULL REFERENCES booster_configs (id),
    pack_variant_index int NOT NULL, -- index into config.boosters[] for the variant drawn
    seed               bytea NOT NULL, -- 32 random bytes from crypto/rand, seeds the ChaCha8 draw
    created_at         timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE pack_open_cards (
    pack_open_id uuid NOT NULL REFERENCES pack_opens (id),
    slot         int NOT NULL, -- position within the pack, 0-indexed
    sheet_name   text NOT NULL, -- which named sheet in the config this slot drew from
    foil         boolean NOT NULL,
    card_id      uuid NOT NULL REFERENCES cards (id),
    PRIMARY KEY (pack_open_id, slot)
);

-- +goose Down
DROP TABLE pack_open_cards;
DROP TABLE pack_opens;
