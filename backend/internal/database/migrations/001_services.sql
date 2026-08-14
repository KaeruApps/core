-- +goose Up
CREATE TABLE services (
    id TEXT PRIMARY KEY,
    service_type TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    internal_url TEXT NOT NULL,
    public_url TEXT NOT NULL DEFAULT '',
    native_apps_url TEXT,
    service_token_hash BYTEA NOT NULL CHECK (octet_length(service_token_hash) = 32),
    database_host TEXT,
    database_port INTEGER CHECK (database_port BETWEEN 1 AND 65535),
    database_name NAME,
    database_username NAME,
    provisioning_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (provisioning_status IN ('pending', 'registered')),
    created_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    CHECK (char_length(service_type) BETWEEN 1 AND 63),
    CHECK (char_length(instance_id) = 36),
    CHECK (char_length(name) BETWEEN 1 AND 128),
    CHECK (char_length(version) BETWEEN 1 AND 64),
    CONSTRAINT services_instance_id_unique UNIQUE (instance_id)
);

CREATE INDEX services_last_seen_at_idx ON services (last_seen_at);

-- +goose Down
DROP TABLE services;
