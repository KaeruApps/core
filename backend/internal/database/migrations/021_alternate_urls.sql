-- +goose Up
-- Alternate URL groups are defined once by Kaeru Core. Every service may then
-- supply its own URL for a group, falling back to its public URL when it does
-- not. This replaces the single native apps URL each service used to carry.
CREATE TABLE alternate_url_groups (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT alternate_url_groups_name_length CHECK (char_length(name) BETWEEN 1 AND 64),
    CONSTRAINT alternate_url_groups_name_trimmed CHECK (name = btrim(name))
);

CREATE UNIQUE INDEX alternate_url_groups_name_unique
    ON alternate_url_groups (lower(name));

CREATE TABLE service_alternate_urls (
    service_id TEXT NOT NULL REFERENCES services (id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES alternate_url_groups (id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    PRIMARY KEY (service_id, group_id),
    CONSTRAINT service_alternate_urls_url_length CHECK (char_length(url) BETWEEN 1 AND 2048),
    CONSTRAINT service_alternate_urls_url_trimmed CHECK (url = btrim(url))
);

-- Preserve any configured native apps URL by moving it into a group.
INSERT INTO alternate_url_groups (name, created_at)
SELECT 'Native', now()
WHERE EXISTS (SELECT 1 FROM services WHERE native_apps_url IS NOT NULL);

INSERT INTO service_alternate_urls (service_id, group_id, url)
SELECT services.id, alternate_url_groups.id, services.native_apps_url
FROM services
JOIN alternate_url_groups ON alternate_url_groups.name = 'Native'
WHERE services.native_apps_url IS NOT NULL;

ALTER TABLE services DROP COLUMN native_apps_url;

-- +goose Down
ALTER TABLE services ADD COLUMN native_apps_url TEXT;

UPDATE services
SET native_apps_url = service_alternate_urls.url
FROM service_alternate_urls
JOIN alternate_url_groups ON alternate_url_groups.id = service_alternate_urls.group_id
WHERE service_alternate_urls.service_id = services.id
  AND alternate_url_groups.name = 'Native';

DROP TABLE service_alternate_urls;
DROP TABLE alternate_url_groups;
