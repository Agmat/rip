-- +goose Up
-- Release date lets the API order sets newest-first once there are more
-- than a handful (bulk import brings in every released set at once).
ALTER TABLE sets ADD COLUMN release_date date;

-- +goose Down
ALTER TABLE sets DROP COLUMN release_date;
