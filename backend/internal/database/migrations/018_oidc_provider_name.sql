-- +goose Up
ALTER TABLE oidc_settings
ADD COLUMN provider_name TEXT NOT NULL DEFAULT 'OIDC'
CHECK (char_length(provider_name) BETWEEN 1 AND 128);

-- +goose Down
ALTER TABLE oidc_settings DROP COLUMN provider_name;
