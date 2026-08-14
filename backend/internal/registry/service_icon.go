package registry

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

var ErrServiceIconNotFound = errors.New("service icon is not available")

type ServiceIconStore interface {
	GetService(ctx context.Context, serviceID string) (ServiceDetails, error)
	GetServiceIcon(ctx context.Context, serviceID string) (ServiceIcon, error)
	UpsertServiceIcon(ctx context.Context, icon ServiceIcon) error
}

type ServiceIconClient interface {
	Fetch(ctx context.Context, internalURL string) (FetchedServiceIcon, error)
}

type ServiceIconManager struct {
	store  ServiceIconStore
	client ServiceIconClient
	now    func() time.Time
}

func NewServiceIconManager(store ServiceIconStore, client ServiceIconClient) *ServiceIconManager {
	return &ServiceIconManager{store: store, client: client, now: time.Now}
}

func (manager *ServiceIconManager) Refresh(ctx context.Context, serviceID string, internalURL string) error {
	if manager.client == nil {
		return ErrServiceIconNotFound
	}
	fetched, err := manager.client.Fetch(ctx, internalURL)
	if err != nil {
		return fmt.Errorf("fetch service icon: %w", err)
	}

	return manager.store.UpsertServiceIcon(ctx, ServiceIcon{
		ServiceID:   serviceID,
		Content:     fetched.Content,
		ContentType: fetched.ContentType,
		ContentHash: sha256.Sum256(fetched.Content),
		SourceETag:  fetched.SourceETag,
		FetchedAt:   manager.now().UTC(),
	})
}

func (manager *ServiceIconManager) Get(ctx context.Context, serviceID string) (ServiceIcon, error) {
	service, err := manager.store.GetService(ctx, serviceID)
	if err != nil {
		return ServiceIcon{}, err
	}
	if service.ServiceType != CoreServiceType && service.RegistrationStatus == "registered" {
		refreshContext, cancelRefresh := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		refreshErr := manager.Refresh(refreshContext, serviceID, service.InternalURL)
		cancelRefresh()
		if refreshErr == nil {
			return manager.store.GetServiceIcon(ctx, serviceID)
		}
	}

	return manager.store.GetServiceIcon(ctx, serviceID)
}
