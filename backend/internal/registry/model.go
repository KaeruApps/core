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

// BackupOption is one kind of backup a service can produce, published by the
// service at /api/core/v1/backup/options.
type BackupOption struct {
	ID          int32  `json:"id"`
	Option      string `json:"option"`
	Default     bool   `json:"default"`
	Description string `json:"description"`
}

type BackupOptionsResponse struct {
	Options []BackupOption `json:"options"`
}

type ServiceRoleMapping struct {
	RoleKey    string   `json:"role_key"`
	OIDCGroups []string `json:"oidc_groups"`
}

// AlternateURLGroup is a named way of reaching the platform, defined once by
// Kaeru Core. Each service may publish its own URL for the group.
type AlternateURLGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ServiceAlternateURL is one group as it applies to a single service. URL is
// empty when the service has not supplied one, in which case callers fall back
// to the service's public URL.
type ServiceAlternateURL struct {
	GroupID int64  `json:"group_id"`
	Group   string `json:"group"`
	URL     string `json:"url"`
}

// AlternateURLInput is a single alternate URL as submitted by the UI. GroupID
// is zero for a group Kaeru Core is creating for the first time.
type AlternateURLInput struct {
	GroupID int64  `json:"group_id"`
	Group   string `json:"group"`
	URL     string `json:"url"`
}

type ServiceDetails struct {
	Service
	AlternateURLs []ServiceAlternateURL `json:"alternate_urls"`
	Roles         []ServiceRole         `json:"roles"`
	RoleMappings  []ServiceRoleMapping  `json:"role_mappings"`
	RoleCatalog   RoleCatalogState      `json:"role_catalog"`
}

type UpdateServiceInput struct {
	PublicURL      string               `json:"public_url"`
	DefaultRoleKey *string              `json:"default_role_key"`
	RoleMappings   []ServiceRoleMapping `json:"role_mappings"`
	AlternateURLs  []AlternateURLInput  `json:"alternate_urls"`
}

// ResolveAlternateURL returns the service's URL for an alternate URL group,
// falling back to its public URL when the service has not supplied one.
func (details ServiceDetails) ResolveAlternateURL(groupID int64) string {
	for _, alternate := range details.AlternateURLs {
		if alternate.GroupID == groupID && alternate.URL != "" {
			return alternate.URL
		}
	}

	return details.PublicURL
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
