-- +goose Up
ALTER TABLE services
ADD COLUMN availability_status TEXT NOT NULL DEFAULT 'unknown'
    CHECK (availability_status IN ('unknown', 'online', 'offline')),
ADD COLUMN health_checked_at TIMESTAMPTZ,
ADD COLUMN health_error TEXT,
ADD COLUMN consecutive_health_failures INTEGER NOT NULL DEFAULT 0
    CHECK (consecutive_health_failures >= 0);

UPDATE services
SET availability_status = CASE
    WHEN service_type = 'core' THEN 'online'
    WHEN registration_status = 'unregistered' THEN 'offline'
    ELSE 'unknown'
END;

-- +goose Down
ALTER TABLE services
DROP COLUMN consecutive_health_failures,
DROP COLUMN health_error,
DROP COLUMN health_checked_at,
DROP COLUMN availability_status;
