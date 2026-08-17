package database

import (
	"context"
	"fmt"

	"github.com/KaeruApps/core/internal/identity"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserDirectoryStore struct {
	pool *pgxpool.Pool
}

func NewUserDirectoryStore(pool *pgxpool.Pool) *UserDirectoryStore {
	return &UserDirectoryStore{pool: pool}
}

func (store *UserDirectoryStore) ListUsers(ctx context.Context) ([]identity.UserSummary, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id, username, display_name, email,
		       CASE
		           WHEN avatar_image IS NOT NULL THEN '/api/v1/users/' || id || '/avatar'
		           ELSE oidc_avatar_url
		       END,
		       disabled_at IS NOT NULL, created_at, last_login_at, last_seen_at
		FROM users
		ORDER BY last_seen_at DESC, username
	`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := []identity.UserSummary{}
	userIndexes := map[string]int{}
	for rows.Next() {
		var user identity.UserSummary
		if err := rows.Scan(
			&user.ID, &user.Username, &user.DisplayName, &user.Email, &user.AvatarURL,
			&user.Disabled, &user.CreatedAt, &user.LastLoginAt, &user.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		user.OIDCGroups = []string{}
		user.Access = []identity.UserServiceAccess{}
		user.RegisteredDevices = []identity.RegisteredDevice{}
		users = append(users, user)
		userIndexes[user.ID] = len(users) - 1
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	groupRows, err := store.pool.Query(ctx, `
		SELECT user_id, group_name
		FROM user_oidc_groups
		ORDER BY user_id, group_name
	`)
	if err != nil {
		return nil, fmt.Errorf("list user OIDC groups: %w", err)
	}
	defer groupRows.Close()
	for groupRows.Next() {
		var userID, group string
		if err := groupRows.Scan(&userID, &group); err != nil {
			return nil, fmt.Errorf("scan user OIDC group: %w", err)
		}
		if index, exists := userIndexes[userID]; exists {
			users[index].OIDCGroups = append(users[index].OIDCGroups, group)
		}
	}
	if err := groupRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user OIDC groups: %w", err)
	}

	accessRows, err := store.pool.Query(ctx, `
		SELECT users.id, services.id, services.name,
		       COALESCE(mapped_role.role_key, default_role.role_key, ''),
		       COALESCE(mapped_role.name, default_role.name, 'No Access')
		FROM users
		CROSS JOIN services
		LEFT JOIN service_roles default_role
		  ON default_role.service_id = services.id
		 AND default_role.role_key = services.default_role_key
		LEFT JOIN LATERAL (
			SELECT roles.role_key, roles.name
			FROM service_roles roles
			JOIN service_role_groups mappings
			  ON mappings.service_id = roles.service_id
			 AND mappings.role_key = roles.role_key
			JOIN user_oidc_groups groups
			  ON groups.user_id = users.id
			 AND groups.group_name = mappings.oidc_group
			WHERE roles.service_id = services.id AND roles.active = TRUE
			ORDER BY roles.priority DESC
			LIMIT 1
		) mapped_role ON TRUE
		ORDER BY users.id, services.created_at, services.name
	`)
	if err != nil {
		return nil, fmt.Errorf("list user service access: %w", err)
	}
	defer accessRows.Close()
	for accessRows.Next() {
		var userID string
		var access identity.UserServiceAccess
		if err := accessRows.Scan(
			&userID, &access.ServiceID, &access.ServiceName, &access.RoleKey, &access.RoleName,
		); err != nil {
			return nil, fmt.Errorf("scan user service access: %w", err)
		}
		if index, exists := userIndexes[userID]; exists {
			users[index].Access = append(users[index].Access, access)
		}
	}
	if err := accessRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user service access: %w", err)
	}

	return users, nil
}
