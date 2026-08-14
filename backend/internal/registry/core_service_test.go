package registry

import (
	"testing"
	"time"
)

func TestNewCoreServiceUsesReservedIdentity(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	service := NewCoreService("0.1.0", "http://localhost:3000", now)

	if service.ID != CoreServiceID || service.InstanceID != CoreServiceInstanceID || service.ServiceType != CoreServiceType {
		t.Fatalf("unexpected Core identity: %#v", service)
	}
	if service.RegistrationStatus != "registered" || service.DatabaseName != "" {
		t.Fatalf("unexpected Core lifecycle: %#v", service)
	}
	roles := CoreRoleCatalog()
	if len(roles) != 1 || roles[0].Key != CoreAdminRoleKey {
		t.Fatalf("unexpected Core role catalog: %#v", roles)
	}
}
