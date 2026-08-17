-- +goose Up
ALTER TABLE users
ADD COLUMN display_name TEXT,
ADD CONSTRAINT users_display_name_length
    CHECK (display_name IS NULL OR char_length(display_name) BETWEEN 1 AND 255);

CREATE TABLE user_preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    time_format TEXT NOT NULL DEFAULT '24h',
    timezone TEXT NOT NULL DEFAULT 'automatic',
    theme TEXT NOT NULL DEFAULT 'dark',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CHECK (time_format IN ('12h', '24h')),
    CHECK (char_length(timezone) BETWEEN 1 AND 255),
    CHECK (theme IN ('dark', 'light')),
    CHECK (updated_at >= created_at)
);

-- +goose Down
DROP TABLE user_preferences;
ALTER TABLE users DROP COLUMN display_name;
