-- +goose Up
ALTER TABLE oidc_settings
ADD COLUMN avatar_claim TEXT NOT NULL DEFAULT 'picture';

-- +goose Down
ALTER TABLE oidc_settings
DROP COLUMN avatar_claim;
