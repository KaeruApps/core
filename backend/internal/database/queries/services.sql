-- name: EnsureCoreService :one
INSERT INTO services (
    id,
    service_type,
    instance_id,
    name,
    version,
    internal_url,
    service_token_hash,
    provisioning_status,
    registration_status,
    availability_status,
    health_checked_at,
    created_at,
    last_seen_at
) VALUES (
    $1, $2, $3, $4, $5, $6, NULL, 'registered', 'registered', 'online', $8, $7, $8
)
ON CONFLICT (service_type) DO UPDATE
SET instance_id = EXCLUDED.instance_id,
    name = EXCLUDED.name,
    version = EXCLUDED.version,
    internal_url = EXCLUDED.internal_url,
    service_token_hash = NULL,
    database_host = NULL,
    database_port = NULL,
    database_name = NULL,
    database_username = NULL,
    default_role_key = NULL,
    provisioning_status = 'registered',
    registration_status = 'registered',
    availability_status = 'online',
    health_checked_at = EXCLUDED.last_seen_at,
    health_error = NULL,
    consecutive_health_failures = 0,
    last_seen_at = EXCLUDED.last_seen_at
RETURNING id;

-- name: ClaimServiceRegistration :one
INSERT INTO services (
    id,
    service_type,
    instance_id,
    name,
    version,
    internal_url,
    public_url,
    native_apps_url,
    service_token_hash,
    registration_status,
    created_at,
    last_seen_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, 'registering', $10, $11
)
ON CONFLICT (service_type) DO UPDATE
SET instance_id = EXCLUDED.instance_id,
    name = EXCLUDED.name,
    version = EXCLUDED.version,
    internal_url = EXCLUDED.internal_url,
    service_token_hash = EXCLUDED.service_token_hash,
    registration_status = 'registering',
    availability_status = 'unknown',
    health_checked_at = NULL,
    health_error = NULL,
    consecutive_health_failures = 0,
    last_seen_at = EXCLUDED.last_seen_at
WHERE services.registration_status = 'unregistered'
RETURNING
    id,
    service_type,
    instance_id,
    name,
    version,
    internal_url,
    public_url,
    native_apps_url,
    default_role_key,
    database_host,
    database_port,
    database_name,
    database_username,
    provisioning_status,
    registration_status,
    availability_status,
    health_checked_at,
    health_error,
    consecutive_health_failures,
    created_at,
    last_seen_at;

-- name: ListServiceRoles :many
SELECT service_id, role_key, name, priority, active
FROM service_roles
WHERE service_id = $1
ORDER BY active DESC, priority DESC, role_key;

-- name: DeactivateServiceRoles :exec
UPDATE service_roles
SET active = FALSE
WHERE service_id = $1;

-- name: UpsertServiceRole :exec
INSERT INTO service_roles (service_id, role_key, name, priority, active)
VALUES ($1, $2, $3, $4, TRUE)
ON CONFLICT (service_id, role_key) DO UPDATE
SET name = EXCLUDED.name,
    priority = EXCLUDED.priority,
    active = TRUE;

-- name: CompleteRoleCatalogRefresh :execrows
UPDATE services
SET role_catalog_refreshed_at = $2,
    role_catalog_refresh_error = NULL
WHERE id = $1;

-- name: RecordRoleCatalogRefreshFailure :execrows
UPDATE services
SET role_catalog_refresh_error = $2
WHERE id = $1;

-- name: ListServiceRoleGroups :many
SELECT groups.service_id, groups.role_key, groups.oidc_group
FROM service_role_groups AS groups
JOIN service_roles AS roles
    ON roles.service_id = groups.service_id
    AND roles.role_key = groups.role_key
WHERE groups.service_id = $1
ORDER BY roles.priority DESC, groups.role_key, groups.oidc_group;

-- name: UpdateServiceConfiguration :execrows
UPDATE services
SET public_url = $2,
    native_apps_url = sqlc.narg(native_apps_url),
    default_role_key = sqlc.narg(default_role_key)
WHERE id = $1;

-- name: DeleteServiceRoleGroups :exec
DELETE FROM service_role_groups
WHERE service_id = $1;

-- name: CreateServiceRoleGroup :exec
INSERT INTO service_role_groups (service_id, role_key, oidc_group)
VALUES ($1, $2, $3);

-- name: CompleteServiceProvisioning :execrows
UPDATE services
SET database_host = $2,
    database_port = $3,
    database_name = $4,
    database_username = $5,
    provisioning_status = 'registered',
    registration_status = 'registered'
WHERE id = $1;

-- name: AbandonServiceRegistration :execrows
UPDATE services
SET service_token_hash = NULL,
    registration_status = 'unregistered',
    availability_status = 'offline',
    health_error = 'service is unregistered',
    consecutive_health_failures = 0
WHERE id = $1
  AND registration_status = 'registering';

-- name: UnregisterService :execrows
UPDATE services
SET service_token_hash = NULL,
    registration_status = 'unregistered',
    availability_status = 'offline',
    health_error = 'service is unregistered',
    consecutive_health_failures = 0,
    role_catalog_refresh_error = 'service is unregistered'
WHERE id = $1
  AND registration_status = 'registered';

-- name: DeleteService :exec
DELETE FROM services
WHERE id = $1;

-- name: GetServiceByInstanceID :one
SELECT
    id,
    service_type,
    instance_id,
    name,
    version,
    internal_url,
    public_url,
    native_apps_url,
    default_role_key,
    database_host,
    database_port,
    database_name,
    database_username,
    provisioning_status,
    registration_status,
    availability_status,
    health_checked_at,
    health_error,
    consecutive_health_failures,
    created_at,
    last_seen_at
FROM services
WHERE instance_id = $1;

-- name: GetServiceByID :one
SELECT
    id,
    service_type,
    instance_id,
    name,
    version,
    internal_url,
    public_url,
    native_apps_url,
    default_role_key,
    role_catalog_refreshed_at,
    role_catalog_refresh_error,
    database_host,
    database_port,
    database_name,
    database_username,
    provisioning_status,
    registration_status,
    availability_status,
    health_checked_at,
    health_error,
    consecutive_health_failures,
    created_at,
    last_seen_at
FROM services
WHERE id = $1;

-- name: ListServices :many
SELECT
    id,
    service_type,
    instance_id,
    name,
    version,
    internal_url,
    public_url,
    native_apps_url,
    default_role_key,
    database_host,
    database_port,
    database_name,
    database_username,
    provisioning_status,
    registration_status,
    availability_status,
    health_checked_at,
    health_error,
    consecutive_health_failures,
    created_at,
    last_seen_at
FROM services
ORDER BY CASE WHEN service_type = 'core' THEN 0 ELSE 1 END, name, id;

-- name: GetServiceTokenHash :one
SELECT service_token_hash
FROM services
WHERE id = $1;

-- name: GetServiceIcon :one
SELECT service_id, content, content_type, content_hash, source_etag, fetched_at
FROM service_icons
WHERE service_id = $1;

-- name: MarkRegisteredServicesHealthUnknown :exec
UPDATE services
SET availability_status = 'unknown',
    health_checked_at = NULL,
    health_error = NULL,
    consecutive_health_failures = 0
WHERE registration_status = 'registered'
  AND service_type <> 'core';

-- name: ListServicesForHealthCheck :many
SELECT id, internal_url
FROM services
WHERE registration_status = 'registered'
  AND service_type <> 'core'
ORDER BY id;

-- name: RecordServiceHealthSuccess :execrows
UPDATE services
SET availability_status = 'online',
    health_checked_at = $2,
    health_error = NULL,
    consecutive_health_failures = 0,
    last_seen_at = $2
WHERE id = $1
  AND registration_status = 'registered';

-- name: RecordServiceHealthUnavailable :execrows
UPDATE services
SET availability_status = 'offline',
    health_checked_at = $2,
    health_error = 'service reported itself unavailable',
    consecutive_health_failures = 0,
    last_seen_at = $2
WHERE id = $1
  AND registration_status = 'registered';

-- name: RecordServiceHealthFailure :execrows
UPDATE services
SET availability_status = CASE
        WHEN availability_status = 'unknown'
          OR consecutive_health_failures + 1 >= $4
        THEN 'offline'
        ELSE availability_status
    END,
    health_checked_at = $2,
    health_error = $3,
    consecutive_health_failures = consecutive_health_failures + 1
WHERE id = $1
  AND registration_status = 'registered';

-- name: UpsertServiceIcon :exec
INSERT INTO service_icons (
    service_id,
    content,
    content_type,
    content_hash,
    source_etag,
    fetched_at
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (service_id) DO UPDATE
SET content = EXCLUDED.content,
    content_type = EXCLUDED.content_type,
    content_hash = EXCLUDED.content_hash,
    source_etag = EXCLUDED.source_etag,
    fetched_at = EXCLUDED.fetched_at;
