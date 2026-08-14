-- +goose Up
ALTER TABLE services
ALTER COLUMN service_token_hash DROP NOT NULL;

ALTER TABLE services
ADD COLUMN registration_status TEXT NOT NULL DEFAULT 'registered'
    CHECK (registration_status IN ('registering', 'registered', 'unregistered'));

ALTER TABLE services
ADD CONSTRAINT services_service_type_unique UNIQUE (service_type);

ALTER TABLE services
ADD CONSTRAINT services_service_type_database_name_length
CHECK (char_length(service_type) <= 57);

-- +goose Down
ALTER TABLE services
DROP CONSTRAINT services_service_type_database_name_length;

ALTER TABLE services
DROP CONSTRAINT services_service_type_unique;

ALTER TABLE services
DROP COLUMN registration_status;

ALTER TABLE services
ALTER COLUMN service_token_hash SET NOT NULL;
