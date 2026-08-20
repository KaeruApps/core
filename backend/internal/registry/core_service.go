package registry

import "time"

const (
	CoreServiceID         = "00000000-0000-0000-0000-000000000001"
	CoreServiceInstanceID = "00000000-0000-0000-0000-000000000002"
	CoreServiceType       = "core"
	CoreServiceName       = "Kaeru Core"
	CoreAdminRoleKey      = "admin"
)

func NewCoreService(version string, internalURL string, now time.Time) Service {
	return Service{
		ID:                 CoreServiceID,
		ServiceType:        CoreServiceType,
		InstanceID:         CoreServiceInstanceID,
		Name:               CoreServiceName,
		Version:            version,
		InternalURL:        internalURL,
		ProvisioningStatus: "registered",
		RegistrationStatus: "registered",
		AvailabilityStatus: "online",
		HealthCheckedAt:    &now,
		CreatedAt:          now,
		LastSeenAt:         now,
	}
}

// CoreBackupOptions is Kaeru Core's own backup catalog. Core does not call the
// service protocol against itself, so its single option is defined here.
func CoreBackupOptions() []BackupOption {
	return []BackupOption{{
		ID:          1,
		Option:      "Full Backup",
		Default:     true,
		Description: "Core configuration, the service registry, users, and shared settings.",
	}}
}

func CoreRoleCatalog() []RoleDefinition {
	return []RoleDefinition{
		{Key: CoreAdminRoleKey, Name: "Administrator", Priority: 100},
	}
}
