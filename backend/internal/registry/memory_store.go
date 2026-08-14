package registry

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"sync"
)

type storedService struct {
	service          Service
	serviceTokenHash [sha256.Size]byte
}

type MemoryStore struct {
	mutex       sync.RWMutex
	byID        map[string]storedService
	instanceID  map[string]string
	serviceType map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		byID:        make(map[string]storedService),
		instanceID:  make(map[string]string),
		serviceType: make(map[string]string),
	}
}

func (store *MemoryStore) ClaimRegistration(_ context.Context, service Service, tokenHash [sha256.Size]byte) (RegistrationClaim, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if _, exists := store.instanceID[service.InstanceID]; exists {
		return RegistrationClaim{}, ErrAlreadyRegistered
	}
	if serviceID, exists := store.serviceType[service.ServiceType]; exists {
		stored := store.byID[serviceID]
		if stored.service.RegistrationStatus != "unregistered" {
			return RegistrationClaim{}, ErrServiceTypeRegistered
		}

		delete(store.instanceID, stored.service.InstanceID)
		stored.service.InstanceID = service.InstanceID
		stored.service.Name = service.Name
		stored.service.Version = service.Version
		stored.service.InternalURL = service.InternalURL
		stored.service.LastSeenAt = service.LastSeenAt
		stored.service.RegistrationStatus = "registering"
		stored.service.AvailabilityStatus = "unknown"
		stored.serviceTokenHash = tokenHash
		store.byID[serviceID] = stored
		store.instanceID[service.InstanceID] = serviceID
		return RegistrationClaim{Service: stored.service, Reused: true}, nil
	}

	service.RegistrationStatus = "registering"
	service.AvailabilityStatus = "unknown"
	store.byID[service.ID] = storedService{service: service, serviceTokenHash: tokenHash}
	store.instanceID[service.InstanceID] = service.ID
	store.serviceType[service.ServiceType] = service.ID
	return RegistrationClaim{Service: service}, nil
}

func (store *MemoryStore) Delete(_ context.Context, serviceID string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	stored, exists := store.byID[serviceID]
	if !exists {
		return nil
	}

	delete(store.byID, serviceID)
	delete(store.instanceID, stored.service.InstanceID)
	delete(store.serviceType, stored.service.ServiceType)
	return nil
}

func (store *MemoryStore) AbandonRegistration(_ context.Context, serviceID string, reused bool) error {
	if !reused {
		return store.Delete(context.Background(), serviceID)
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()
	stored, exists := store.byID[serviceID]
	if !exists {
		return ErrServiceNotFound
	}
	stored.service.RegistrationStatus = "unregistered"
	stored.service.AvailabilityStatus = "offline"
	stored.serviceTokenHash = [sha256.Size]byte{}
	store.byID[serviceID] = stored
	return nil
}

func (store *MemoryStore) UnregisterService(_ context.Context, serviceID string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	stored, exists := store.byID[serviceID]
	if !exists {
		return ErrServiceNotFound
	}
	stored.service.RegistrationStatus = "unregistered"
	stored.service.AvailabilityStatus = "offline"
	stored.serviceTokenHash = [sha256.Size]byte{}
	store.byID[serviceID] = stored
	return nil
}

func (store *MemoryStore) CompleteProvisioning(_ context.Context, serviceID string, database DatabaseCredentials) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	stored, exists := store.byID[serviceID]
	if !exists {
		return ErrServiceNotFound
	}

	stored.service.DatabaseHost = database.Host
	stored.service.DatabasePort = database.Port
	stored.service.DatabaseName = database.Database
	stored.service.DatabaseUsername = database.Username
	stored.service.ProvisioningStatus = "registered"
	stored.service.RegistrationStatus = "registered"
	store.byID[serviceID] = stored
	return nil
}

func (store *MemoryStore) FindByInstanceID(instanceID string) (Service, bool) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	serviceID, exists := store.instanceID[instanceID]
	if !exists {
		return Service{}, false
	}

	return store.byID[serviceID].service, true
}

func (store *MemoryStore) Authenticate(serviceID string, serviceToken string) bool {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	stored, exists := store.byID[serviceID]
	if !exists {
		return false
	}

	candidateHash := sha256.Sum256([]byte(serviceToken))
	return subtle.ConstantTimeCompare(stored.serviceTokenHash[:], candidateHash[:]) == 1
}
