-- +goose Up
CREATE UNIQUE INDEX users_username_unique
ON users (LOWER(username));

-- +goose Down
DROP INDEX users_username_unique;
