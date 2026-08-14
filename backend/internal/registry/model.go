package registry

import "time"

// Service is a Kaeru service instance known to Core.
type Service struct {
	ID                 string     `json:"id"`
	ServiceType        string     `json:"service_type"`
	InstanceID         string     `json:"instance_id"`
	Name               string     `json:"name"`
	Version            string     `json:"version"`
	InternalURL        string     `json:"internal_url"`
	PublicURL          string     `json:"public_url"`
	NativeAppsURL      string     `json:"native_apps_url,omitempty"`
	DefaultRoleKey     *string    `json:"default_role_key"`
	DatabaseHost       string     `json:"database_host,omitempty"`
	DatabasePort       uint16     `json:"database_port,omitempty"`
	DatabaseName       string     `json:"database_name,omitempty"`
	DatabaseUsername   string     `json:"database_username,omitempty"`
	ProvisioningStatus string     `json:"provisioning_status"`
	RegistrationStatus string     `json:"registration_status"`
	AvailabilityStatus string     `json:"availability_status"`
	HealthCheckedAt    *time.Time `json:"health_checked_at"`
	HealthError        string     `json:"health_error,omitempty"`
	HealthFailures     int32      `json:"consecutive_health_failures"`
	CreatedAt          time.Time  `json:"created_at"`
	LastSeenAt         time.Time  `json:"last_seen_at"`
}

// ServiceRole is a role definition supplied by a registered service.
type ServiceRole struct {
	ServiceID string `json:"service_id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	Priority  int32  `json:"priority"`
	Active    bool   `json:"active"`
}

type RoleDefinition struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Priority int32  `json:"priority"`
}

type RoleCatalogResponse struct {
	Roles []RoleDefinition `json:"roles"`
}

type RoleCatalogState struct {
	Status      string     `json:"status"`
	RefreshedAt *time.Time `json:"refreshed_at"`
	Stale       bool       `json:"stale"`
}

type ServiceRoleMapping struct {
	RoleKey    string   `json:"role_key"`
	OIDCGroups []string `json:"oidc_groups"`
}

type ServiceDetails struct {
	Service
	Roles        []ServiceRole        `json:"roles"`
	RoleMappings []ServiceRoleMapping `json:"role_mappings"`
	RoleCatalog  RoleCatalogState     `json:"role_catalog"`
}

type UpdateServiceInput struct {
	PublicURL      string               `json:"public_url"`
	NativeAppsURL  *string              `json:"native_apps_url"`
	DefaultRoleKey *string              `json:"default_role_key"`
	RoleMappings   []ServiceRoleMapping `json:"role_mappings"`
}

// ResolvedNativeAppsURL returns the native-app URL when configured and falls
// back to the regular public URL otherwise.
func (service Service) ResolvedNativeAppsURL() string {
	if service.NativeAppsURL != "" {
		return service.NativeAppsURL
	}

	return service.PublicURL
}

type RegistrationInput struct {
	ServiceType string `json:"service_type"`
	InstanceID  string `json:"instance_id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	InternalURL string `json:"internal_url"`
}

type DatabaseCredentials struct {
	Host     string `json:"host"`
	Port     uint16 `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegistrationResult struct {
	Status       string              `json:"status"`
	ServiceID    string              `json:"service_id"`
	ServiceToken string              `json:"service_token"`
	Database     DatabaseCredentials `json:"database"`
}

type ServiceIcon struct {
	ServiceID   string
	Content     []byte
	ContentType string
	ContentHash [32]byte
	SourceETag  string
	FetchedAt   time.Time
}

type FetchedServiceIcon struct {
	Content     []byte
	ContentType string
	SourceETag  string
}
