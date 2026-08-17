package database

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/KaeruApps/core/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IdentityStore struct {
	pool *pgxpool.Pool
}

func NewIdentityStore(pool *pgxpool.Pool) *IdentityStore {
	return &IdentityStore{pool: pool}
}

func (store *IdentityStore) PrincipalBySessionHash(
	ctx context.Context,
	tokenHash [sha256.Size]byte,
	now time.Time,
) (identity.Principal, bool, error) {
	principal := identity.Principal{ServiceRoles: map[string]string{}}
	var coreAdministrator bool
	err := store.pool.QueryRow(ctx, `
		SELECT users.id, users.oidc_subject, users.username, users.display_name, users.email,
		       CASE WHEN users.avatar_image IS NOT NULL
		            THEN '/api/v1/users/me/avatar'
		            ELSE users.oidc_avatar_url
		       END,
		       EXISTS (
		           SELECT 1
		           FROM services
		           JOIN service_roles roles
		             ON roles.service_id = services.id
		            AND roles.role_key = 'admin'
		            AND roles.active = TRUE
		           JOIN service_role_groups mappings
		             ON mappings.service_id = roles.service_id
		            AND mappings.role_key = roles.role_key
		           JOIN user_oidc_groups user_groups
		             ON user_groups.user_id = users.id
		            AND user_groups.group_name = mappings.oidc_group
		           WHERE services.service_type = 'core'
		       )
		FROM user_sessions sessions
		JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash = $1
		  AND sessions.revoked_at IS NULL
		  AND sessions.expires_at > $2
		  AND users.disabled_at IS NULL
	`, tokenHash[:], now).Scan(
		&principal.ID,
		&principal.Subject,
		&principal.Name,
		&principal.DisplayName,
		&principal.Email,
		&principal.AvatarURL,
		&coreAdministrator,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Principal{}, false, nil
	}
	if err != nil {
		return identity.Principal{}, false, fmt.Errorf("authenticate user session: %w", err)
	}

	if coreAdministrator {
		principal.ServiceRoles["core"] = "admin"
	}

	_, err = store.pool.Exec(ctx, `
		WITH touched_session AS (
			UPDATE user_sessions
			SET last_seen_at = $2
			WHERE token_hash = $1 AND last_seen_at < $2::timestamptz - INTERVAL '5 minutes'
			RETURNING user_id
		)
		UPDATE users
		SET last_seen_at = $2
		WHERE id IN (SELECT user_id FROM touched_session)
	`, tokenHash[:], now)
	if err != nil {
		return identity.Principal{}, false, fmt.Errorf("update session activity: %w", err)
	}
	return principal, true, nil
}

func (store *IdentityStore) RevokeSessionHash(
	ctx context.Context,
	tokenHash [sha256.Size]byte,
	revokedAt time.Time,
	reason string,
) error {
	_, err := store.pool.Exec(ctx, `
		UPDATE user_sessions
		SET revoked_at = $2, revocation_reason = $3
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, tokenHash[:], revokedAt, reason)
	if err != nil {
		return fmt.Errorf("revoke user session: %w", err)
	}
	return nil
}
