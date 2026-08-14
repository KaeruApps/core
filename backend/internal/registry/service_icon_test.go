package registry

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubIconStore struct {
	service ServiceDetails
	icon    ServiceIcon
}

func (store *stubIconStore) GetService(context.Context, string) (ServiceDetails, error) {
	return store.service, nil
}

func (store *stubIconStore) GetServiceIcon(context.Context, string) (ServiceIcon, error) {
	if len(store.icon.Content) == 0 {
		return ServiceIcon{}, ErrServiceIconNotFound
	}
	return store.icon, nil
}

func (store *stubIconStore) UpsertServiceIcon(_ context.Context, icon ServiceIcon) error {
	store.icon = icon
	return nil
}

type stubIconClient struct {
	icon FetchedServiceIcon
	err  error
}

func (client *stubIconClient) Fetch(context.Context, string) (FetchedServiceIcon, error) {
	return client.icon, client.err
}

func TestServiceIconManagerRefreshesAndCachesIcon(t *testing.T) {
	store := &stubIconStore{service: ServiceDetails{Service: Service{
		ID: "service-id", InternalURL: "http://mock-service:3101", RegistrationStatus: "registered",
	}}}
	client := &stubIconClient{icon: FetchedServiceIcon{
		Content: []byte("icon"), ContentType: "image/svg+xml", SourceETag: `"v1"`,
	}}
	manager := NewServiceIconManager(store, client)
	fetchedAt := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return fetchedAt }

	icon, err := manager.Get(context.Background(), "service-id")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(icon.Content) != "icon" || icon.SourceETag != `"v1"` || !icon.FetchedAt.Equal(fetchedAt) {
		t.Fatalf("unexpected cached icon: %#v", icon)
	}
}

func TestServiceIconManagerUsesCacheWhenServiceIsOffline(t *testing.T) {
	store := &stubIconStore{
		service: ServiceDetails{Service: Service{ID: "service-id", RegistrationStatus: "unregistered"}},
		icon:    ServiceIcon{ServiceID: "service-id", Content: []byte("cached"), ContentType: "image/png"},
	}
	manager := NewServiceIconManager(store, &stubIconClient{err: errors.New("offline")})

	icon, err := manager.Get(context.Background(), "service-id")
	if err != nil || string(icon.Content) != "cached" {
		t.Fatalf("Get() icon = %#v, error = %v", icon, err)
	}
}
