-- +goose Up
INSERT INTO service_role_groups (service_id, role_key, oidc_group)
SELECT services.id, 'admin', settings.admin_group
FROM oidc_settings AS settings
JOIN services ON services.service_type = 'core'
JOIN service_roles
    ON service_roles.service_id = services.id
    AND service_roles.role_key = 'admin'
ON CONFLICT DO NOTHING;

ALTER TABLE oidc_settings DROP COLUMN admin_group;

-- +goose Down
ALTER TABLE oidc_settings ADD COLUMN admin_group TEXT;

UPDATE oidc_settings
SET admin_group = mapping.oidc_group
FROM (
    SELECT groups.oidc_group
    FROM service_role_groups AS groups
    JOIN services ON services.id = groups.service_id
    WHERE services.service_type = 'core'
      AND groups.role_key = 'admin'
    ORDER BY groups.oidc_group
    LIMIT 1
) AS mapping
WHERE oidc_settings.singleton = TRUE;

UPDATE oidc_settings
SET admin_group = 'kaeru-admins'
WHERE admin_group IS NULL;

ALTER TABLE oidc_settings ALTER COLUMN admin_group SET NOT NULL;
