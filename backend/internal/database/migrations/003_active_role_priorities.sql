-- +goose Up
ALTER TABLE service_roles
DROP CONSTRAINT service_roles_service_id_priority_key;

CREATE UNIQUE INDEX service_roles_active_priority_unique
ON service_roles (service_id, priority)
WHERE active;

-- +goose Down
DROP INDEX service_roles_active_priority_unique;

ALTER TABLE service_roles
ADD CONSTRAINT service_roles_service_id_priority_key
UNIQUE (service_id, priority);
