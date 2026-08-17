-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    oidc_issuer TEXT NOT NULL,
    oidc_subject TEXT NOT NULL,
    username TEXT NOT NULL,
    email TEXT,
    email_verified BOOLEAN,
    oidc_avatar_url TEXT,
    disabled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    last_login_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    UNIQUE (oidc_issuer, oidc_subject),
    CHECK (char_length(oidc_issuer) BETWEEN 1 AND 2048),
    CHECK (char_length(oidc_subject) BETWEEN 1 AND 512),
    CHECK (char_length(username) BETWEEN 1 AND 255),
    CHECK (email IS NULL OR char_length(email) BETWEEN 1 AND 320),
    CHECK (email IS NOT NULL OR email_verified IS NULL),
    CHECK (oidc_avatar_url IS NULL OR char_length(oidc_avatar_url) BETWEEN 1 AND 4096),
    CHECK (updated_at >= created_at),
    CHECK (last_login_at >= created_at),
    CHECK (last_seen_at >= created_at),
    CHECK (disabled_at IS NULL OR disabled_at >= created_at)
);

CREATE TABLE user_oidc_groups (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_name TEXT NOT NULL,
    PRIMARY KEY (user_id, group_name),
    CHECK (char_length(group_name) BETWEEN 1 AND 255)
);

CREATE INDEX user_oidc_groups_group_name_idx
ON user_oidc_groups (group_name, user_id);

CREATE TABLE user_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revocation_reason TEXT,
    user_agent TEXT,
    ip_address INET,
    CHECK (last_seen_at >= created_at),
    CHECK (last_seen_at <= expires_at),
    CHECK (expires_at > created_at),
    CHECK (revoked_at IS NULL OR revoked_at >= created_at),
    CHECK (revocation_reason IS NULL OR char_length(revocation_reason) BETWEEN 1 AND 255),
    CHECK (revocation_reason IS NULL OR revoked_at IS NOT NULL),
    CHECK (user_agent IS NULL OR char_length(user_agent) BETWEEN 1 AND 1024)
);

CREATE INDEX user_sessions_active_user_idx
ON user_sessions (user_id, expires_at)
WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE user_sessions;
DROP TABLE user_oidc_groups;
DROP TABLE users;
