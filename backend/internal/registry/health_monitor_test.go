package registry

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type stubHealthStore struct {
	targets     []HealthCheckTarget
	prepared    bool
	status      string
	failures    int32
	checkedAt   time.Time
	healthError string
}

func (store *stubHealthStore) PrepareHealthChecks(context.Context) error {
	store.prepared = true
	store.status = "unknown"
	store.failures = 0
	return nil
}

func (store *stubHealthStore) ListHealthCheckTargets(context.Context) ([]HealthCheckTarget, error) {
	return store.targets, nil
}

func (store *stubHealthStore) RecordHealthSuccess(_ context.Context, _ string, checkedAt time.Time) error {
	store.status = "online"
	store.failures = 0
	store.healthError = ""
	store.checkedAt = checkedAt
	return nil
}

func (store *stubHealthStore) RecordHealthUnavailable(_ context.Context, _ string, checkedAt time.Time) error {
	store.status = "offline"
	store.failures = 0
	store.healthError = "service reported itself unavailable"
	store.checkedAt = checkedAt
	return nil
}

func (store *stubHealthStore) RecordHealthFailure(_ context.Context, _ string, checkedAt time.Time, message string, threshold int32) error {
	store.failures++
	if store.status == "unknown" || store.failures >= threshold {
		store.status = "offline"
	}
	store.healthError = message
	store.checkedAt = checkedAt
	return nil
}

type stubHealthClient struct {
	available bool
	err       error
}

func (client *stubHealthClient) Check(context.Context, string) (bool, error) {
	return client.available, client.err
}

func testHealthMonitor(store ServiceHealthStore, client ServiceHealthClient) *HealthMonitor {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHealthMonitor(store, client, logger)
}

func TestHealthMonitorDefaults(t *testing.T) {
	monitor := testHealthMonitor(&stubHealthStore{}, &stubHealthClient{})
	if monitor.interval != 15*time.Second || monitor.timeout != 2*time.Second || monitor.failureThreshold != 2 {
		t.Fatalf("unexpected health defaults: %#v", monitor)
	}
}

func TestHealthMonitorRecordsAvailability(t *testing.T) {
	store := &stubHealthStore{targets: []HealthCheckTarget{{ServiceID: "service-id", InternalURL: "http://service:8080"}}}
	client := &stubHealthClient{available: true}
	monitor := testHealthMonitor(store, client)
	checkedAt := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	monitor.now = func() time.Time { return checkedAt }

	if err := monitor.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() error = %v", err)
	}
	if store.status != "online" || !store.checkedAt.Equal(checkedAt) {
		t.Fatalf("unexpected health state: %#v", store)
	}

	client.available = false
	if err := monitor.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() unavailable error = %v", err)
	}
	if store.status != "offline" || store.failures != 0 {
		t.Fatalf("unexpected unavailable state: %#v", store)
	}
}

func TestHealthMonitorDebouncesFailuresForOnlineService(t *testing.T) {
	store := &stubHealthStore{
		targets: []HealthCheckTarget{{ServiceID: "service-id", InternalURL: "http://service:8080"}},
		status:  "online",
	}
	client := &stubHealthClient{err: errors.New("connection refused")}
	monitor := testHealthMonitor(store, client)

	if err := monitor.CheckOnce(context.Background()); err != nil {
		t.Fatalf("first CheckOnce() error = %v", err)
	}
	if store.status != "online" || store.failures != 1 {
		t.Fatalf("first failure state: %#v", store)
	}
	if err := monitor.CheckOnce(context.Background()); err != nil {
		t.Fatalf("second CheckOnce() error = %v", err)
	}
	if store.status != "offline" || store.failures != 2 {
		t.Fatalf("second failure state: %#v", store)
	}
}
