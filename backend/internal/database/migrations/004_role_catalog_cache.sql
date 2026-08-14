-- +goose Up
ALTER TABLE services
ADD COLUMN role_catalog_refreshed_at TIMESTAMPTZ,
ADD COLUMN role_catalog_refresh_error TEXT;

-- +goose Down
ALTER TABLE services
DROP COLUMN role_catalog_refresh_error,
DROP COLUMN role_catalog_refreshed_at;
