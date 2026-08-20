package registry

import (
	"context"
	"errors"
	"testing"
)

type stubBackupServiceLister struct {
	services []Service
	err      error
}

func (stub stubBackupServiceLister) ListServices(context.Context) ([]Service, error) {
	return stub.services, stub.err
}

type stubBackupOptionsClient struct {
	byURL map[string][]BackupOption
	err   error
}

func (stub stubBackupOptionsClient) Fetch(_ context.Context, internalURL string) ([]BackupOption, error) {
	if stub.err != nil {
		return nil, stub.err
	}
	options, exists := stub.byURL[internalURL]
	if !exists {
		return nil, errors.New("no options published")
	}
	return options, nil
}

func registeredService(id, serviceType, name, internalURL string) Service {
	return Service{
		ID: id, ServiceType: serviceType, Name: name, InternalURL: internalURL,
		RegistrationStatus: "registered", AvailabilityStatus: "online",
	}
}

func TestBackupDirectoryPutsCoreFirstWithItsOwnOptions(t *testing.T) {
	store := stubBackupServiceLister{services: []Service{
		registeredService("upload", "upload", "Kaeru Upload", "http://upload:8080"),
		registeredService(CoreServiceID, CoreServiceType, CoreServiceName, "http://core:8080"),
		registeredService("archive", "archive", "Kaeru Archive", "http://archive:8080"),
	}}
	client := stubBackupOptionsClient{byURL: map[string][]BackupOption{
		"http://upload:8080":  {{ID: 1, Option: "Full Backup", Default: true}},
		"http://archive:8080": {{ID: 7, Option: "Everything"}},
	}}

	entries, err := NewBackupDirectory(store, client).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	if entries[0].ServiceType != CoreServiceType {
		t.Errorf("first entry is %q, want Kaeru Core", entries[0].ServiceName)
	}
	if len(entries[0].Options) != 1 || entries[0].Options[0].Option != "Full Backup" {
		t.Errorf("Core options = %+v, want a single Full Backup option", entries[0].Options)
	}
	if !entries[0].Options[0].Default {
		t.Error("Core's only option should be the default")
	}
	// Remaining services follow in name order.
	if entries[1].ServiceName != "Kaeru Archive" || entries[2].ServiceName != "Kaeru Upload" {
		t.Errorf("services out of order: %q then %q", entries[1].ServiceName, entries[2].ServiceName)
	}
}

func TestBackupDirectoryMarksUnreachableServicesUnavailable(t *testing.T) {
	offline := registeredService("offline", "offline", "Offline Service", "http://offline:8080")
	offline.AvailabilityStatus = "offline"
	unregistered := registeredService("gone", "gone", "Unregistered Service", "http://gone:8080")
	unregistered.RegistrationStatus = "unregistered"
	broken := registeredService("broken", "broken", "Broken Service", "http://broken:8080")

	store := stubBackupServiceLister{services: []Service{offline, unregistered, broken}}
	client := stubBackupOptionsClient{byURL: map[string][]BackupOption{}}

	entries, err := NewBackupDirectory(store, client).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	for _, entry := range entries {
		if entry.Available {
			t.Errorf("%s should not be available", entry.ServiceName)
		}
		if entry.Unavailable == "" {
			t.Errorf("%s should explain why it is unavailable", entry.ServiceName)
		}
		if entry.Options == nil {
			t.Errorf("%s should carry an empty option list, not null", entry.ServiceName)
		}
	}
}

func TestBackupDirectoryRejectsInvalidPublishedOptions(t *testing.T) {
	store := stubBackupServiceLister{services: []Service{
		registeredService("upload", "upload", "Kaeru Upload", "http://upload:8080"),
	}}
	// Two defaults is not a usable catalog.
	client := stubBackupOptionsClient{byURL: map[string][]BackupOption{
		"http://upload:8080": {
			{ID: 1, Option: "Full Backup", Default: true},
			{ID: 2, Option: "Database Only", Default: true},
		},
	}}

	entries, err := NewBackupDirectory(store, client).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if entries[0].Available {
		t.Error("a service publishing an invalid catalog should not be available")
	}
}

func TestValidateBackupOptions(t *testing.T) {
	valid := []BackupOption{
		{ID: 1, Option: "Full Backup", Default: true, Description: "Everything."},
		{ID: 2, Option: "Database Only"},
	}
	if err := ValidateBackupOptions(valid); err != nil {
		t.Errorf("ValidateBackupOptions(valid) error = %v", err)
	}

	cases := map[string][]BackupOption{
		"empty":            {},
		"zero id":          {{ID: 0, Option: "Full Backup"}},
		"duplicate id":     {{ID: 1, Option: "A"}, {ID: 1, Option: "B"}},
		"duplicate name":   {{ID: 1, Option: "A"}, {ID: 2, Option: "A"}},
		"blank name":       {{ID: 1, Option: "   "}},
		"untrimmed name":   {{ID: 1, Option: " Full Backup "}},
		"two defaults":     {{ID: 1, Option: "A", Default: true}, {ID: 2, Option: "B", Default: true}},
		"overlong name":    {{ID: 1, Option: string(make([]byte, maxBackupOptionName+1))}},
		"overlong descrip": {{ID: 1, Option: "A", Description: string(make([]byte, maxBackupOptionDescription+1))}},
	}
	for name, options := range cases {
		if err := ValidateBackupOptions(options); err == nil {
			t.Errorf("ValidateBackupOptions(%s) accepted an invalid catalog", name)
		}
	}
}
