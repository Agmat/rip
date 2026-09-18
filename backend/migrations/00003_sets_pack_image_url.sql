-- +goose Up
ALTER TABLE sets ADD COLUMN pack_image_url text;

-- +goose Down
ALTER TABLE sets DROP COLUMN pack_image_url;
