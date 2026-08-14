package database

import (
	"context"
	"strings"
	"testing"

	"github.com/KaeruApps/core/internal/registry"
)

func TestServiceDatabaseName(t *testing.T) {
	service := registry.Service{
		ServiceType: strings.Repeat("a", 57),
	}

	name, err := serviceDatabaseName(service)
	if err != nil {
		t.Fatalf("serviceDatabaseName() error = %v", err)
	}
	if len(name) > 63 {
		t.Fatalf("database name is %d characters, want at most 63", len(name))
	}
	if !managedDatabaseNamePattern.MatchString(name) {
		t.Fatalf("database name %q is not recognized as managed", name)
	}
	if name != "kaeru_"+service.ServiceType {
		t.Fatalf("database name = %q, want deterministic name", name)
	}
}

func TestDeprovisionRejectsUnmanagedDatabase(t *testing.T) {
	provisioner := NewServiceDatabaseProvisioner(nil, "kaeru-postgres", 5432)
	err := provisioner.Deprovision(context.Background(), registry.DatabaseCredentials{
		Database: "kaeru",
		Username: "kaeru",
	})
	if err == nil {
		t.Fatal("expected unmanaged database to be rejected")
	}
}

func TestNewDatabasePassword(t *testing.T) {
	password, err := newDatabasePassword()
	if err != nil {
		t.Fatalf("newDatabasePassword() error = %v", err)
	}
	if len(password) < 40 {
		t.Fatalf("password is unexpectedly short: %d", len(password))
	}
	if strings.ContainsAny(password, "'\"") {
		t.Fatalf("password contains SQL quoting characters: %q", password)
	}
}
