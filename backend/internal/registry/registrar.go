package registry

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

var (
	ErrAlreadyRegistered       = errors.New("service instance is already registered")
	ErrServiceTypeRegistered   = errors.New("service type is already registered")
	ErrServiceNotFound         = errors.New("service is not registered")
	ErrBuiltInService          = errors.New("built-in service operation is not allowed")
	ErrProvisioningUnavailable = errors.New("service database provisioning is unavailable")
)

type Store interface {
	ClaimRegistration(ctx context.Context, service Service, serviceTokenHash [sha256.Size]byte) (RegistrationClaim, error)
	CompleteProvisioning(ctx context.Context, serviceID string, database DatabaseCredentials) error
	AbandonRegistration(ctx context.Context, serviceID string, reused bool) error
	Delete(ctx context.Context, serviceID string) error
}

type DatabaseProvisioner interface {
	Provision(ctx context.Context, service Service) (DatabaseCredentials, error)
	Suspend(ctx context.Context, database DatabaseCredentials) error
	Deprovision(ctx context.Context, database DatabaseCredentials) error
}

type ServiceIconRefresher interface {
	Refresh(ctx context.Context, serviceID string, internalURL string) error
}

type ServiceHealthRefresher interface {
	Check(ctx context.Context, serviceID string, internalURL string) error
}

type RegistrationClaim struct {
	Service Service
	Reused  bool
}

type Registrar struct {
	store       Store
	provisioner DatabaseProvisioner
	icon        ServiceIconRefresher
	health      ServiceHealthRefresher
	now         func() time.Time
}

func (registrar *Registrar) WithHealthRefresher(health ServiceHealthRefresher) *Registrar {
	registrar.health = health
	return registrar
}

func NewRegistrar(store Store, provisioner DatabaseProvisioner, icon ...ServiceIconRefresher) *Registrar {
	registrar := &Registrar{
		store:       store,
		provisioner: provisioner,
		now:         time.Now,
	}
	if len(icon) > 0 {
		registrar.icon = icon[0]
	}
	return registrar
}

func (registrar *Registrar) Register(ctx context.Context, input RegistrationInput) (RegistrationResult, error) {
	if err := ValidateRegistration(input); err != nil {
		return RegistrationResult{}, err
	}

	serviceID, err := newUUID()
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("generate service ID: %w", err)
	}
	serviceToken, err := newServiceToken()
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("generate service token: %w", err)
	}

	now := registrar.now().UTC()
	service := Service{
		ID:                 serviceID,
		ServiceType:        input.ServiceType,
		InstanceID:         input.InstanceID,
		Name:               input.Name,
		Version:            input.Version,
		InternalURL:        input.InternalURL,
		ProvisioningStatus: "pending",
		CreatedAt:          now,
		LastSeenAt:         now,
	}
	tokenHash := sha256.Sum256([]byte(serviceToken))

	claim, err := registrar.store.ClaimRegistration(ctx, service, tokenHash)
	if err != nil {
		return RegistrationResult{}, err
	}
	service = claim.Service

	database, err := registrar.provisioner.Provision(ctx, service)
	if err != nil {
		cleanupContext, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancelCleanup()
		if cleanupErr := registrar.store.AbandonRegistration(cleanupContext, service.ID, claim.Reused); cleanupErr != nil {
			return RegistrationResult{}, fmt.Errorf("provision database: %w; remove incomplete registration: %v", err, cleanupErr)
		}
		return RegistrationResult{}, fmt.Errorf("provision database: %w", err)
	}
	if err := registrar.store.CompleteProvisioning(ctx, service.ID, database); err != nil {
		cleanupContext, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancelCleanup()
		var databaseCleanupErr error
		if claim.Reused {
			databaseCleanupErr = registrar.provisioner.Suspend(cleanupContext, database)
		} else {
			databaseCleanupErr = registrar.provisioner.Deprovision(cleanupContext, database)
		}
		abandonErr := registrar.store.AbandonRegistration(cleanupContext, service.ID, claim.Reused)
		return RegistrationResult{}, errors.Join(
			fmt.Errorf("complete service registration: %w", err),
			databaseCleanupErr,
			abandonErr,
		)
	}

	status := "created"
	if claim.Reused {
		status = "reattached"
	}
	if registrar.icon != nil {
		iconContext, cancelIcon := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		_ = registrar.icon.Refresh(iconContext, service.ID, service.InternalURL)
		cancelIcon()
	}
	if registrar.health != nil {
		healthContext, cancelHealth := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		_ = registrar.health.Check(healthContext, service.ID, service.InternalURL)
		cancelHealth()
	}
	return RegistrationResult{
		Status:       status,
		ServiceID:    service.ID,
		ServiceToken: serviceToken,
		Database:     database,
	}, nil
}

func newServiceToken() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}

	return "kaeru_" + base64.RawURLEncoding.EncodeToString(random), nil
}

func newUUID() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}

	random[6] = (random[6] & 0x0f) | 0x40
	random[8] = (random[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		random[0:4],
		random[4:6],
		random[6:8],
		random[8:10],
		random[10:16],
	), nil
}
