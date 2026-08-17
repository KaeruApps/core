-- +goose Up
CREATE TABLE oidc_settings (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    issuer_url TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret TEXT NOT NULL,
    additional_scopes TEXT[] NOT NULL DEFAULT '{}',
    username_claim TEXT NOT NULL,
    groups_claim TEXT NOT NULL,
    admin_group TEXT NOT NULL,
    button_text TEXT NOT NULL,
    button_image BYTEA,
    button_image_content_type TEXT
        CHECK (button_image_content_type IS NULL OR button_image_content_type IN ('image/jpeg', 'image/png')),
    updated_at TIMESTAMPTZ NOT NULL,
    CHECK ((button_image IS NULL) = (button_image_content_type IS NULL)),
    CHECK (button_image IS NULL OR octet_length(button_image) BETWEEN 1 AND 1048576)
);

CREATE TABLE oidc_login_attempts (
    state_hash BYTEA PRIMARY KEY CHECK (octet_length(state_hash) = 32),
    code_verifier TEXT NOT NULL,
    nonce TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE oidc_login_attempts;
DROP TABLE oidc_settings;
