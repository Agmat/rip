-- +goose Up
-- Stored so GET /v1/sets reads a 32-char hash instead of detoasting and
-- hashing every set's pack image on each request. Postgres keeps it in
-- sync with pack_image.
ALTER TABLE sets ADD COLUMN pack_image_hash text GENERATED ALWAYS AS (md5(pack_image)) STORED;

-- +goose Down
ALTER TABLE sets DROP COLUMN pack_image_hash;
