package registry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var validRegistration = RegistrationInput{
	ServiceType: "upload",
	InstanceID:  "019c2a46-7f5d-7ca2-9f4a-ae191ca84322",
	Name:        "Upload Archiver",
	Version:     "0.1.0",
	InternalURL: "http://kaeru-upload:8080",
}

type stubProvisioner struct {
	credentials      DatabaseCredentials
	err              error
	calls            int
	deprovisionCalls int
	suspendCalls     int
}

type stubIconRefresher struct {
	serviceID   string
	internalURL string
}

type stubHealthRefresher struct {
	serviceID   string
	internalURL string
}

func (refresher *stubHealthRefresher) Check(_ context.Context, serviceID string, internalURL string) error {
	refresher.serviceID = serviceID
	refresher.internalURL = internalURL
	return nil
}

func (refresher *stubIconRefresher) Refresh(_ context.Context, serviceID string, internalURL string) error {
	refresher.serviceID = serviceID
	refresher.internalURL = internalURL
	return nil
}

func (provisioner *stubProvisioner) Provision(_ context.Context, _ Service) (DatabaseCredentials, error) {
	provisioner.calls++
	return provisioner.credentials, provisioner.err
}

func (provisioner *stubProvisioner) Deprovision(_ context.Context, _ DatabaseCredentials) error {
	provisioner.deprovisionCalls++
	return nil
}

func (provisioner *stubProvisioner) Suspend(_ context.Context, _ DatabaseCredentials) error {
	provisioner.suspendCalls++
	return nil
}

type failingCompleteStore struct {
	*MemoryStore
}

func (store *failingCompleteStore) CompleteProvisioning(context.Context, string, DatabaseCredentials) error {
	return errors.New("complete failed")
}

func TestRegisterCreatesServiceAndCredentials(t *testing.T) {
	store := NewMemoryStore()
	provisioner := &stubProvisioner{credentials: DatabaseCredentials{
		Host: "kaeru-postgres", Port: 5432, Database: "kaeru_upload_a83f",
		Username: "kaeru_upload_a83f", Password: "generated-secret",
	}}
	registrar := NewRegistrar(store, provisioner)
	registrar.now = func() time.Time { return time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC) }

	result, err := registrar.Register(context.Background(), validRegistration)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if result.Status != "created" || result.Database != provisioner.credentials {
		t.Fatalf("unexpected registration result: %#v", result)
	}
	if !strings.HasPrefix(result.ServiceToken, "kaeru_") {
		t.Fatalf("unexpected service token format: %q", result.ServiceToken)
	}
	if !store.Authenticate(result.ServiceID, result.ServiceToken) {
		t.Fatal("generated service token did not authenticate")
	}

	service, found := store.FindByInstanceID(validRegistration.InstanceID)
	if !found || service.ID != result.ServiceID {
		t.Fatalf("registered service was not stored: %#v", service)
	}
	if service.PublicURL != "" {
		t.Fatalf("registration must not set Core-managed public URLs: %#v", service)
	}
	if service.ProvisioningStatus != "registered" || service.DatabaseName != provisioner.credentials.Database {
		t.Fatalf("database metadata was not stored: %#v", service)
	}
	if service.RegistrationStatus != "registered" {
		t.Fatalf("service registration status = %q", service.RegistrationStatus)
	}
}

func TestRegisterRefreshesServiceIcon(t *testing.T) {
	store := NewMemoryStore()
	refresher := &stubIconRefresher{}
	registrar := NewRegistrar(store, &stubProvisioner{}, refresher)

	result, err := registrar.Register(context.Background(), validRegistration)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if refresher.serviceID != result.ServiceID || refresher.internalURL != validRegistration.InternalURL {
		t.Fatalf("unexpected icon refresh: %#v", refresher)
	}
}

func TestRegisterChecksServiceHealth(t *testing.T) {
	store := NewMemoryStore()
	refresher := &stubHealthRefresher{}
	registrar := NewRegistrar(store, &stubProvisioner{}).WithHealthRefresher(refresher)

	result, err := registrar.Register(context.Background(), validRegistration)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if refresher.serviceID != result.ServiceID || refresher.internalURL != validRegistration.InternalURL {
		t.Fatalf("unexpected health refresh: %#v", refresher)
	}
}

func TestRegisterRejectsDuplicateInstance(t *testing.T) {
	store := NewMemoryStore()
	provisioner := &stubProvisioner{}
	registrar := NewRegistrar(store, provisioner)

	if _, err := registrar.Register(context.Background(), validRegistration); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if _, err := registrar.Register(context.Background(), validRegistration); !errors.Is(err, ErrAlreadyRegistered) {
		t.Fatalf("second Register() error = %v, want ErrAlreadyRegistered", err)
	}
	if provisioner.calls != 1 {
		t.Fatalf("provisioner called %d times, want 1", provisioner.calls)
	}
}

func TestRegisterRejectsDuplicateServiceType(t *testing.T) {
	store := NewMemoryStore()
	registrar := NewRegistrar(store, &stubProvisioner{})
	if _, err := registrar.Register(context.Background(), validRegistration); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}

	second := validRegistration
	second.InstanceID = "119c2a46-7f5d-7ca2-9f4a-ae191ca84322"
	if _, err := registrar.Register(context.Background(), second); !errors.Is(err, ErrServiceTypeRegistered) {
		t.Fatalf("second Register() error = %v, want ErrServiceTypeRegistered", err)
	}
}

func TestRegisterReattachesUnregisteredService(t *testing.T) {
	store := NewMemoryStore()
	credentials := DatabaseCredentials{Database: "kaeru_upload", Username: "kaeru_upload"}
	registrar := NewRegistrar(store, &stubProvisioner{credentials: credentials})
	first, err := registrar.Register(context.Background(), validRegistration)
	if err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if err := store.UnregisterService(context.Background(), first.ServiceID); err != nil {
		t.Fatalf("UnregisterService() error = %v", err)
	}

	secondInput := validRegistration
	secondInput.InstanceID = "119c2a46-7f5d-7ca2-9f4a-ae191ca84322"
	second, err := registrar.Register(context.Background(), secondInput)
	if err != nil {
		t.Fatalf("second Register() error = %v", err)
	}
	if second.Status != "reattached" || second.ServiceID != first.ServiceID {
		t.Fatalf("unexpected reattachment result: %#v", second)
	}
}

func TestRegisterRemovesClaimWhenProvisioningFails(t *testing.T) {
	store := NewMemoryStore()
	registrar := NewRegistrar(store, &stubProvisioner{err: ErrProvisioningUnavailable})

	_, err := registrar.Register(context.Background(), validRegistration)
	if !errors.Is(err, ErrProvisioningUnavailable) {
		t.Fatalf("Register() error = %v, want ErrProvisioningUnavailable", err)
	}
	if _, found := store.FindByInstanceID(validRegistration.InstanceID); found {
		t.Fatal("incomplete registration remained in the store")
	}
}

func TestRegisterDeprovisionsDatabaseWhenCompletionFails(t *testing.T) {
	store := &failingCompleteStore{MemoryStore: NewMemoryStore()}
	provisioner := &stubProvisioner{credentials: DatabaseCredentials{
		Database: "kaeru_upload_a83f",
		Username: "kaeru_upload_a83f",
	}}
	registrar := NewRegistrar(store, provisioner)

	if _, err := registrar.Register(context.Background(), validRegistration); err == nil {
		t.Fatal("expected registration completion to fail")
	}
	if provisioner.deprovisionCalls != 1 {
		t.Fatalf("Deprovision() called %d times, want 1", provisioner.deprovisionCalls)
	}
	if _, found := store.FindByInstanceID(validRegistration.InstanceID); found {
		t.Fatal("failed registration remained in the store")
	}
}

func TestValidateRegistrationRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*RegistrationInput)
		field string
	}{
		{name: "service type", alter: func(input *RegistrationInput) { input.ServiceType = "Upload Service" }, field: "service_type"},
		{name: "instance ID", alter: func(input *RegistrationInput) { input.InstanceID = "not-a-uuid" }, field: "instance_id"},
		{name: "name", alter: func(input *RegistrationInput) { input.Name = " " }, field: "name"},
		{name: "version", alter: func(input *RegistrationInput) { input.Version = "" }, field: "version"},
		{name: "internal URL", alter: func(input *RegistrationInput) { input.InternalURL = "kaeru-upload:8080" }, field: "internal_url"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validRegistration
			test.alter(&input)
			err := ValidateRegistration(input)

			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("ValidateRegistration() error = %v, want field %q", err, test.field)
			}
		})
	}
}

func TestValidateRegistrationRejectsReservedCoreType(t *testing.T) {
	input := validRegistration
	input.ServiceType = CoreServiceType

	err := ValidateRegistration(input)
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Field != "service_type" {
		t.Fatalf("ValidateRegistration() error = %v", err)
	}
}
