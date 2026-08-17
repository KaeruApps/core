-- +goose Up
ALTER TABLE oidc_settings
ADD COLUMN access_urls TEXT[] NOT NULL DEFAULT '{}',
ADD COLUMN display_name_claim TEXT NOT NULL DEFAULT '';

UPDATE oidc_settings AS settings
SET access_urls = ARRAY[core.public_url]
FROM services AS core
WHERE core.service_type = 'core'
  AND core.public_url <> '';

UPDATE oidc_settings
SET access_urls = ARRAY[regexp_replace(redirect_uri, '/api/v1/auth/oidc/callback$', '')]
WHERE cardinality(access_urls) = 0;

ALTER TABLE oidc_settings
ADD CONSTRAINT oidc_settings_access_urls_present CHECK (cardinality(access_urls) > 0),
ADD CONSTRAINT oidc_settings_display_name_claim_length CHECK (char_length(display_name_claim) <= 128);

-- +goose Down
ALTER TABLE oidc_settings
DROP CONSTRAINT oidc_settings_display_name_claim_length,
DROP CONSTRAINT oidc_settings_access_urls_present,
DROP COLUMN display_name_claim,
DROP COLUMN access_urls;
