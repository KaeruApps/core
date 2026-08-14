package registry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	defaultHealthCheckInterval = 15 * time.Second
	defaultHealthCheckTimeout  = 2 * time.Second
	defaultHealthCheckWorkers  = 4
	defaultFailureThreshold    = 2
)

type HealthCheckTarget struct {
	ServiceID   string
	InternalURL string
}

type ServiceHealthStore interface {
	PrepareHealthChecks(ctx context.Context) error
	ListHealthCheckTargets(ctx context.Context) ([]HealthCheckTarget, error)
	RecordHealthSuccess(ctx context.Context, serviceID string, checkedAt time.Time) error
	RecordHealthUnavailable(ctx context.Context, serviceID string, checkedAt time.Time) error
	RecordHealthFailure(ctx context.Context, serviceID string, checkedAt time.Time, message string, threshold int32) error
}

type ServiceHealthClient interface {
	Check(ctx context.Context, internalURL string) (bool, error)
}

type HealthMonitor struct {
	store            ServiceHealthStore
	client           ServiceHealthClient
	logger           *slog.Logger
	interval         time.Duration
	timeout          time.Duration
	workers          int
	failureThreshold int32
	now              func() time.Time
}

func NewHealthMonitor(store ServiceHealthStore, client ServiceHealthClient, logger *slog.Logger) *HealthMonitor {
	if logger == nil {
		logger = slog.Default()
	}
	return &HealthMonitor{
		store:            store,
		client:           client,
		logger:           logger,
		interval:         defaultHealthCheckInterval,
		timeout:          defaultHealthCheckTimeout,
		workers:          defaultHealthCheckWorkers,
		failureThreshold: defaultFailureThreshold,
		now:              time.Now,
	}
}

func (monitor *HealthMonitor) Run(ctx context.Context) {
	if err := monitor.store.PrepareHealthChecks(ctx); err != nil && !errors.Is(err, context.Canceled) {
		monitor.logger.Error("Service health state could not be prepared", "error", err)
	}

	for {
		if err := monitor.CheckOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			monitor.logger.Error("Service health check cycle failed", "error", err)
		}

		timer := time.NewTimer(monitor.interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func (monitor *HealthMonitor) CheckOnce(ctx context.Context) error {
	targets, err := monitor.store.ListHealthCheckTargets(ctx)
	if err != nil {
		return fmt.Errorf("list services for health checks: %w", err)
	}

	semaphore := make(chan struct{}, monitor.workers)
	errorsChannel := make(chan error, len(targets))
	var waitGroup sync.WaitGroup
	for _, target := range targets {
		target := target
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				errorsChannel <- ctx.Err()
				return
			}
			if err := monitor.Check(ctx, target.ServiceID, target.InternalURL); err != nil {
				errorsChannel <- err
			}
		}()
	}
	waitGroup.Wait()
	close(errorsChannel)

	var cycleErrors []error
	for err := range errorsChannel {
		cycleErrors = append(cycleErrors, err)
	}
	return errors.Join(cycleErrors...)
}

func (monitor *HealthMonitor) Check(ctx context.Context, serviceID string, internalURL string) error {
	checkContext, cancelCheck := context.WithTimeout(ctx, monitor.timeout)
	available, checkErr := monitor.client.Check(checkContext, internalURL)
	cancelCheck()
	if ctx.Err() != nil {
		return ctx.Err()
	}

	checkedAt := monitor.now().UTC()
	if checkErr != nil {
		message := checkErr.Error()
		if len(message) > 1024 {
			message = message[:1024]
		}
		if err := monitor.store.RecordHealthFailure(ctx, serviceID, checkedAt, message, monitor.failureThreshold); err != nil {
			return fmt.Errorf("record health failure for service %q: %w", serviceID, err)
		}
		return nil
	}
	if !available {
		if err := monitor.store.RecordHealthUnavailable(ctx, serviceID, checkedAt); err != nil {
			return fmt.Errorf("record unavailable service %q: %w", serviceID, err)
		}
		return nil
	}
	if err := monitor.store.RecordHealthSuccess(ctx, serviceID, checkedAt); err != nil {
		return fmt.Errorf("record healthy service %q: %w", serviceID, err)
	}
	return nil
}
