-- +goose Up
ALTER TABLE services
ADD COLUMN default_role_key TEXT
    CHECK (default_role_key IS NULL OR char_length(default_role_key) BETWEEN 1 AND 64);

CREATE TABLE service_roles (
    service_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    role_key TEXT NOT NULL,
    name TEXT NOT NULL,
    priority INTEGER NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (service_id, role_key),
    UNIQUE (service_id, priority),
    CHECK (char_length(role_key) BETWEEN 1 AND 64),
    CHECK (char_length(name) BETWEEN 1 AND 128),
    CHECK (priority >= 0)
);

CREATE TABLE service_role_groups (
    service_id TEXT NOT NULL,
    role_key TEXT NOT NULL,
    oidc_group TEXT NOT NULL,
    PRIMARY KEY (service_id, role_key, oidc_group),
    FOREIGN KEY (service_id, role_key)
        REFERENCES service_roles(service_id, role_key)
        ON DELETE CASCADE,
    CHECK (char_length(oidc_group) BETWEEN 1 AND 255)
);

CREATE INDEX service_role_groups_lookup_idx
ON service_role_groups (service_id, oidc_group);

-- +goose Down
DROP TABLE service_role_groups;
DROP TABLE service_roles;
ALTER TABLE services DROP COLUMN default_role_key;
