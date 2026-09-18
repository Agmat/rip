-- +goose Up
ALTER TABLE sets DROP COLUMN pack_image_url;
ALTER TABLE sets ADD COLUMN pack_image bytea;

-- +goose Down
ALTER TABLE sets DROP COLUMN pack_image;
ALTER TABLE sets ADD COLUMN pack_image_url text;
