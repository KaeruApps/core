-- +goose Up
ALTER TABLE oidc_login_attempts
DROP CONSTRAINT oidc_login_attempts_purpose,
ADD CONSTRAINT oidc_login_attempts_purpose
    CHECK (purpose IN ('setup', 'login', 'settings_verification'));

CREATE TABLE oidc_pending_settings (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    provider_name TEXT NOT NULL,
    issuer_url TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret TEXT NOT NULL,
    additional_scopes TEXT[] NOT NULL DEFAULT '{}',
    username_claim TEXT NOT NULL,
    display_name_claim TEXT NOT NULL DEFAULT '',
    avatar_claim TEXT NOT NULL DEFAULT '',
    groups_claim TEXT NOT NULL,
    admin_groups TEXT[] NOT NULL,
    access_urls TEXT[] NOT NULL,
    button_text TEXT NOT NULL,
    button_image BYTEA,
    button_image_content_type TEXT
        CHECK (button_image_content_type IS NULL OR button_image_content_type IN ('image/jpeg', 'image/png')),
    redirect_uri TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CHECK (cardinality(admin_groups) > 0),
    CHECK (cardinality(access_urls) > 0),
    CHECK ((button_image IS NULL) = (button_image_content_type IS NULL)),
    CHECK (button_image IS NULL OR octet_length(button_image) BETWEEN 1 AND 1048576)
);

-- +goose Down
DROP TABLE oidc_pending_settings;

ALTER TABLE oidc_login_attempts
DROP CONSTRAINT oidc_login_attempts_purpose,
ADD CONSTRAINT oidc_login_attempts_purpose
    CHECK (purpose IN ('setup', 'login'));
