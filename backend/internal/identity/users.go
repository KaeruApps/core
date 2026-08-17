package identity

import (
	"context"
	"time"
)

type UserServiceAccess struct {
	ServiceID   string `json:"service_id"`
	ServiceName string `json:"service_name"`
	RoleKey     string `json:"role_key,omitempty"`
	RoleName    string `json:"role_name"`
}

type RegisteredDevice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserSummary struct {
	ID                string              `json:"id"`
	Username          string              `json:"username"`
	DisplayName       *string             `json:"display_name"`
	Email             *string             `json:"email"`
	AvatarURL         *string             `json:"avatar_url"`
	Disabled          bool                `json:"disabled"`
	OIDCGroups        []string            `json:"oidc_groups"`
	Access            []UserServiceAccess `json:"access"`
	RegisteredDevices []RegisteredDevice  `json:"registered_devices"`
	CreatedAt         time.Time           `json:"created_at"`
	LastLoginAt       time.Time           `json:"last_login_at"`
	LastSeenAt        time.Time           `json:"last_seen_at"`
}

type UserDirectoryStore interface {
	ListUsers(context.Context) ([]UserSummary, error)
}

type UserDirectory struct {
	store UserDirectoryStore
}

func NewUserDirectory(store UserDirectoryStore) *UserDirectory {
	return &UserDirectory{store: store}
}

func (directory *UserDirectory) List(ctx context.Context) ([]UserSummary, error) {
	return directory.store.ListUsers(ctx)
}
