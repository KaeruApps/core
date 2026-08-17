-- +goose Up
ALTER TABLE oidc_login_attempts
ADD COLUMN purpose TEXT NOT NULL DEFAULT 'setup',
ADD CONSTRAINT oidc_login_attempts_purpose CHECK (purpose IN ('setup', 'login'));

-- +goose Down
ALTER TABLE oidc_login_attempts DROP COLUMN purpose;
