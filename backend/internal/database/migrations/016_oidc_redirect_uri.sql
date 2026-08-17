-- +goose Up
ALTER TABLE oidc_settings
ADD COLUMN redirect_uri TEXT NOT NULL DEFAULT 'http://localhost:3000/api/v1/auth/oidc/callback';

UPDATE oidc_settings
SET redirect_uri = COALESCE(
    (
        SELECT NULLIF(RTRIM(public_url, '/'), '') || '/api/v1/auth/oidc/callback'
        FROM services
        WHERE service_type = 'core'
        LIMIT 1
    ),
    (
        SELECT redirect_uri
        FROM oidc_login_attempts
        WHERE purpose = 'login'
        ORDER BY created_at DESC
        LIMIT 1
    ),
    redirect_uri
);

ALTER TABLE oidc_settings ALTER COLUMN redirect_uri DROP DEFAULT;

-- +goose Down
ALTER TABLE oidc_settings DROP COLUMN redirect_uri;
