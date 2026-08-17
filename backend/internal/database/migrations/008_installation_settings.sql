-- +goose Up
CREATE TABLE installation_settings (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    setup_state TEXT NOT NULL DEFAULT 'required'
        CHECK (setup_state IN ('required', 'configuring', 'restoring', 'ready')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO installation_settings (singleton, setup_state)
VALUES (TRUE, 'required');

-- +goose Down
DROP TABLE installation_settings;
