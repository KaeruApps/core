-- +goose Up
ALTER TABLE users
ADD COLUMN avatar_image BYTEA,
ADD COLUMN avatar_image_content_type TEXT,
ADD CONSTRAINT users_avatar_image_pair
    CHECK ((avatar_image IS NULL) = (avatar_image_content_type IS NULL)),
ADD CONSTRAINT users_avatar_image_size
    CHECK (avatar_image IS NULL OR octet_length(avatar_image) BETWEEN 1 AND 1048576),
ADD CONSTRAINT users_avatar_image_content_type
    CHECK (avatar_image_content_type IS NULL OR avatar_image_content_type IN ('image/jpeg', 'image/png'));

-- +goose Down
ALTER TABLE users
DROP COLUMN avatar_image,
DROP COLUMN avatar_image_content_type;
