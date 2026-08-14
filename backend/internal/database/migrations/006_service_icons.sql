-- +goose Up
CREATE TABLE service_icons (
    service_id TEXT PRIMARY KEY REFERENCES services(id) ON DELETE CASCADE,
    content BYTEA NOT NULL,
    content_type TEXT NOT NULL
        CHECK (content_type IN ('image/png', 'image/svg+xml')),
    content_hash BYTEA NOT NULL
        CHECK (octet_length(content_hash) = 32),
    source_etag TEXT,
    fetched_at TIMESTAMPTZ NOT NULL,
    CHECK (octet_length(content) BETWEEN 1 AND 262144)
);

-- +goose Down
DROP TABLE service_icons;
