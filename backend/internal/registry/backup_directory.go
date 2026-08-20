package registry

import (
	"context"
	"sort"
	"sync"
	"time"
)

const backupOptionsFetchTimeout = 5 * time.Second

// ServiceBackupOptions is one service's entry in the backup directory.
type ServiceBackupOptions struct {
	ServiceID   string         `json:"service_id"`
	ServiceType string         `json:"service_type"`
	ServiceName string         `json:"service_name"`
	Available   bool           `json:"available"`
	Unavailable string         `json:"unavailable_reason,omitempty"`
	Options     []BackupOption `json:"options"`
}

type BackupOptionsClient interface {
	Fetch(ctx context.Context, internalURL string) ([]BackupOption, error)
}

type BackupServiceLister interface {
	ListServices(ctx context.Context) ([]Service, error)
}

// BackupDirectory reports which services can take part in a platform backup and
// what each one offers. Kaeru Core is always first: it is the service that
// coordinates the backup, and a platform backup without Core is not useful.
type BackupDirectory struct {
	store  BackupServiceLister
	client BackupOptionsClient
}

func NewBackupDirectory(store BackupServiceLister, client BackupOptionsClient) *BackupDirectory {
	return &BackupDirectory{store: store, client: client}
}

func (directory *BackupDirectory) List(ctx context.Context) ([]ServiceBackupOptions, error) {
	services, err := directory.store.ListServices(ctx)
	if err != nil {
		return nil, err
	}

	entries := make([]ServiceBackupOptions, 0, len(services))
	remote := make([]int, 0, len(services))
	for _, service := range services {
		entry := ServiceBackupOptions{
			ServiceID:   service.ID,
			ServiceType: service.ServiceType,
			ServiceName: service.Name,
			Options:     []BackupOption{},
		}
		switch {
		case service.ServiceType == CoreServiceType:
			entry.Available = true
			entry.Options = CoreBackupOptions()
		case service.RegistrationStatus != "registered":
			entry.Unavailable = "The service is not registered."
		case service.AvailabilityStatus == "offline":
			entry.Unavailable = "The service is offline."
		default:
			remote = append(remote, len(entries))
		}
		entries = append(entries, entry)
	}

	if directory.client != nil {
		var waitGroup sync.WaitGroup
		for _, index := range remote {
			waitGroup.Add(1)
			go func(index int, internalURL string) {
				defer waitGroup.Done()
				fetchContext, cancelFetch := context.WithTimeout(ctx, backupOptionsFetchTimeout)
				defer cancelFetch()
				options, err := directory.client.Fetch(fetchContext, internalURL)
				if err == nil {
					err = ValidateBackupOptions(options)
				}
				if err != nil {
					entries[index].Unavailable = "The service did not publish usable backup options."
					return
				}
				entries[index].Available = true
				entries[index].Options = options
			}(index, services[index].InternalURL)
		}
		waitGroup.Wait()
	}

	// Core first, then the remaining services by name so the list is stable.
	sort.SliceStable(entries, func(first, second int) bool {
		if (entries[first].ServiceType == CoreServiceType) != (entries[second].ServiceType == CoreServiceType) {
			return entries[first].ServiceType == CoreServiceType
		}
		return entries[first].ServiceName < entries[second].ServiceName
	})

	return entries, nil
}
