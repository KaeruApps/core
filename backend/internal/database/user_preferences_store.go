package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/KaeruApps/core/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserPreferencesStore struct {
	pool *pgxpool.Pool
}

func NewUserPreferencesStore(pool *pgxpool.Pool) *UserPreferencesStore {
	return &UserPreferencesStore{pool: pool}
}

func (store *UserPreferencesStore) GetUserPreferences(ctx context.Context, userID string) (identity.UserPreferences, error) {
	var preferences identity.UserPreferences
	err := store.pool.QueryRow(ctx, `
		SELECT users.username, users.display_name, users.email,
		       COALESCE(preferences.time_format, '24h'),
		       COALESCE(preferences.timezone, 'automatic'),
		       COALESCE(preferences.theme, 'dark')
		FROM users
		LEFT JOIN user_preferences preferences ON preferences.user_id = users.id
		WHERE users.id = $1
	`, userID).Scan(
		&preferences.Username,
		&preferences.DisplayName,
		&preferences.Email,
		&preferences.TimeFormat,
		&preferences.Timezone,
		&preferences.Theme,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.UserPreferences{}, identity.ErrUserNotFound
	}
	if err != nil {
		return identity.UserPreferences{}, fmt.Errorf("load user preferences: %w", err)
	}
	return preferences, nil
}

func (store *UserPreferencesStore) UpdateUserPreferences(
	ctx context.Context,
	userID string,
	preferences identity.UserPreferences,
	now time.Time,
) (identity.UserPreferences, error) {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return identity.UserPreferences{}, fmt.Errorf("begin user preferences transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err := lockUsernameAllocation(ctx, transaction); err != nil {
		return identity.UserPreferences{}, err
	}

	result, err := transaction.Exec(ctx, `
		UPDATE users
		SET username = $2,
		    display_name = $3,
		    email_verified = CASE WHEN email IS NOT DISTINCT FROM $4::text THEN email_verified ELSE NULL END,
		    email = $4,
		    updated_at = $5
		WHERE id = $1
	`, userID, preferences.Username, preferences.DisplayName, preferences.Email, now)
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "users_username_unique" {
			return identity.UserPreferences{}, identity.ErrUsernameTaken
		}
		return identity.UserPreferences{}, fmt.Errorf("update user profile: %w", err)
	}
	if result.RowsAffected() != 1 {
		return identity.UserPreferences{}, identity.ErrUserNotFound
	}

	_, err = transaction.Exec(ctx, `
		INSERT INTO user_preferences (
			user_id, time_format, timezone, theme, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			time_format = EXCLUDED.time_format,
			timezone = EXCLUDED.timezone,
			theme = EXCLUDED.theme,
			updated_at = EXCLUDED.updated_at
	`, userID, preferences.TimeFormat, preferences.Timezone, preferences.Theme, now)
	if err != nil {
		return identity.UserPreferences{}, fmt.Errorf("save user preferences: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return identity.UserPreferences{}, fmt.Errorf("commit user preferences: %w", err)
	}
	return preferences, nil
}

func (store *UserPreferencesStore) GetUserAvatar(ctx context.Context, userID string) (identity.UserAvatar, error) {
	var avatar identity.UserAvatar
	err := store.pool.QueryRow(ctx, `
		SELECT avatar_image, avatar_image_content_type, updated_at
		FROM users
		WHERE id = $1 AND avatar_image IS NOT NULL
	`, userID).Scan(&avatar.Content, &avatar.ContentType, &avatar.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.UserAvatar{}, identity.ErrUserAvatarNotFound
	}
	if err != nil {
		return identity.UserAvatar{}, fmt.Errorf("load user avatar: %w", err)
	}
	return avatar, nil
}

func (store *UserPreferencesStore) UpdateUserAvatar(ctx context.Context, userID string, avatar identity.UserAvatar) error {
	result, err := store.pool.Exec(ctx, `
		UPDATE users
		SET avatar_image = $2, avatar_image_content_type = $3, updated_at = $4
		WHERE id = $1
	`, userID, avatar.Content, avatar.ContentType, avatar.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update user avatar: %w", err)
	}
	if result.RowsAffected() != 1 {
		return identity.ErrUserNotFound
	}
	return nil
}
