package registry

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

var ErrCoreAdminVerificationRequired = errors.New("Kaeru Core administrator mappings require OIDC verification")

const (
	maxRoleMappings  = 100
	maxOIDCGroups    = 1000
	maxOIDCGroupName = 255
	maxServiceRoles  = 100
	maxRoleKeyLength = 64
	maxRoleName      = 128

	maxBackupOptions           = 50
	maxBackupOptionName        = 128
	maxBackupOptionDescription = 512
)

var roleKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

type RoleCatalogClient interface {
	Fetch(ctx context.Context, internalURL string) ([]RoleDefinition, error)
}

type ServiceDatabaseLifecycle interface {
	Suspend(ctx context.Context, database DatabaseCredentials) error
	Resume(ctx context.Context, database DatabaseCredentials) error
}

type ServiceConfigurationStore interface {
	ListServices(ctx context.Context) ([]Service, error)
	GetService(ctx context.Context, serviceID string) (ServiceDetails, error)
	UpdateService(ctx context.Context, serviceID string, input UpdateServiceInput) (ServiceDetails, error)
	SyncServiceRoles(ctx context.Context, serviceID string, roles []RoleDefinition, refreshedAt time.Time) error
	RecordRoleCatalogFailure(ctx context.Context, serviceID string, message string) error
	UnregisterService(ctx context.Context, serviceID string) error
}

type ServiceManager struct {
	store             ServiceConfigurationStore
	roleCatalogClient RoleCatalogClient
	databaseLifecycle ServiceDatabaseLifecycle
	roleRefreshes     singleflight.Group
	now               func() time.Time
}

func NewServiceManager(store ServiceConfigurationStore, roleCatalogClient RoleCatalogClient, databaseLifecycle ServiceDatabaseLifecycle) *ServiceManager {
	return &ServiceManager{
		store:             store,
		roleCatalogClient: roleCatalogClient,
		databaseLifecycle: databaseLifecycle,
		now:               time.Now,
	}
}

func (manager *ServiceManager) List(ctx context.Context) ([]Service, error) {
	return manager.store.ListServices(ctx)
}

func (manager *ServiceManager) Get(ctx context.Context, serviceID string) (ServiceDetails, error) {
	service, err := manager.store.GetService(ctx, serviceID)
	if err != nil {
		return ServiceDetails{}, err
	}
	if manager.roleCatalogClient == nil || service.RegistrationStatus != "registered" || service.ServiceType == CoreServiceType {
		return service, nil
	}

	_, refreshErr, _ := manager.roleRefreshes.Do(serviceID, func() (any, error) {
		refreshContext, cancelRefresh := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancelRefresh()

		roles, err := manager.roleCatalogClient.Fetch(refreshContext, service.InternalURL)
		if err == nil {
			err = ValidateRoleCatalog(roles)
		}
		if err != nil {
			message := err.Error()
			if len(message) > 1024 {
				message = message[:1024]
			}
			return nil, manager.store.RecordRoleCatalogFailure(refreshContext, serviceID, message)
		}

		return nil, manager.store.SyncServiceRoles(refreshContext, serviceID, roles, manager.now().UTC())
	})
	if refreshErr != nil {
		return ServiceDetails{}, refreshErr
	}

	return manager.store.GetService(ctx, serviceID)
}

func (manager *ServiceManager) Unregister(ctx context.Context, serviceID string) (ServiceDetails, error) {
	service, err := manager.store.GetService(ctx, serviceID)
	if err != nil {
		return ServiceDetails{}, err
	}
	if service.RegistrationStatus == "unregistered" {
		return service, nil
	}
	if service.ServiceType == CoreServiceType {
		return ServiceDetails{}, ErrBuiltInService
	}
	if service.RegistrationStatus != "registered" {
		return ServiceDetails{}, fmt.Errorf("service registration is currently in progress")
	}
	if manager.databaseLifecycle == nil {
		return ServiceDetails{}, fmt.Errorf("service database lifecycle is unavailable")
	}

	database := DatabaseCredentials{
		Host:     service.DatabaseHost,
		Port:     service.DatabasePort,
		Database: service.DatabaseName,
		Username: service.DatabaseUsername,
	}
	if err := manager.databaseLifecycle.Suspend(ctx, database); err != nil {
		return ServiceDetails{}, fmt.Errorf("suspend service database access: %w", err)
	}
	if err := manager.store.UnregisterService(ctx, serviceID); err != nil {
		resumeErr := manager.databaseLifecycle.Resume(context.WithoutCancel(ctx), database)
		return ServiceDetails{}, errors.Join(fmt.Errorf("unregister service: %w", err), resumeErr)
	}

	return manager.store.GetService(ctx, serviceID)
}

func (manager *ServiceManager) Update(ctx context.Context, serviceID string, input UpdateServiceInput) (ServiceDetails, error) {
	service, err := manager.store.GetService(ctx, serviceID)
	if err != nil {
		return ServiceDetails{}, err
	}
	if service.ServiceType == CoreServiceType && input.DefaultRoleKey != nil {
		return ServiceDetails{}, &ValidationError{Field: "default_role_key", Message: "Kaeru Core must default to No Access."}
	}
	if service.ServiceType == CoreServiceType {
		input.PublicURL = strings.TrimRight(input.PublicURL, "/")
		parsed, parseErr := url.ParseRequestURI(input.PublicURL)
		if parseErr != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
			return ServiceDetails{}, &ValidationError{Field: "public_url", Message: "The Kaeru Core Application URL must be an HTTP or HTTPS origin without a path, query, or fragment."}
		}
	}
	knownGroups := make(map[int64]string, len(service.AlternateURLs))
	for _, alternate := range service.AlternateURLs {
		knownGroups[alternate.GroupID] = alternate.Group
	}
	if err := ValidateServiceUpdate(input, service.Roles, service.ServiceType == CoreServiceType, knownGroups); err != nil {
		return ServiceDetails{}, err
	}
	if service.ServiceType == CoreServiceType && !slices.Equal(
		sortedRoleGroups(service.RoleMappings, CoreAdminRoleKey),
		sortedRoleGroups(input.RoleMappings, CoreAdminRoleKey),
	) {
		return ServiceDetails{}, ErrCoreAdminVerificationRequired
	}

	return manager.store.UpdateService(ctx, serviceID, input)
}

func sortedRoleGroups(mappings []ServiceRoleMapping, roleKey string) []string {
	groups := []string{}
	for _, mapping := range mappings {
		if mapping.RoleKey == roleKey {
			groups = append(groups, mapping.OIDCGroups...)
		}
	}
	slices.Sort(groups)
	return slices.Compact(groups)
}

func ValidateServiceUpdate(input UpdateServiceInput, roles []ServiceRole, isCore bool, knownGroups map[int64]string) error {
	if err := ValidatePublicURL(input.PublicURL); err != nil {
		return err
	}
	if err := ValidateAlternateURLs(input.AlternateURLs, isCore, knownGroups); err != nil {
		return err
	}

	activeRoles := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		if role.Active {
			activeRoles[role.Key] = struct{}{}
		}
	}
	if input.DefaultRoleKey != nil {
		if _, exists := activeRoles[*input.DefaultRoleKey]; !exists {
			return invalid("default_role_key", "The default user role must be one the service provides.")
		}
	}
	if len(input.RoleMappings) > maxRoleMappings {
		return invalid("role_mappings", fmt.Sprintf("There can be at most %d role mappings.", maxRoleMappings))
	}

	mappedRoles := make(map[string]struct{}, len(input.RoleMappings))
	totalGroups := 0
	for mappingIndex, mapping := range input.RoleMappings {
		field := fmt.Sprintf("role_mappings[%d]", mappingIndex)
		if _, exists := activeRoles[mapping.RoleKey]; !exists {
			return invalid(field+".role_key", "Each role mapping must use a role the service provides.")
		}
		if _, exists := mappedRoles[mapping.RoleKey]; exists {
			return invalid(field+".role_key", "Each role can only have one role mapping.")
		}
		mappedRoles[mapping.RoleKey] = struct{}{}
		if len(mapping.OIDCGroups) == 0 {
			return invalid(field+".oidc_groups", "Give every role mapping at least one OIDC group.")
		}

		groups := make(map[string]struct{}, len(mapping.OIDCGroups))
		for groupIndex, group := range mapping.OIDCGroups {
			groupField := fmt.Sprintf("%s.oidc_groups[%d]", field, groupIndex)
			if strings.TrimSpace(group) == "" {
				return invalid(groupField, "OIDC group names must not be empty.")
			}
			if group != strings.TrimSpace(group) || len(group) > maxOIDCGroupName {
				return invalid(groupField, "OIDC group names must be at most 255 characters without leading or trailing whitespace.")
			}
			if _, exists := groups[group]; exists {
				return invalid(groupField, "OIDC group names must be unique within a role mapping.")
			}
			groups[group] = struct{}{}
			totalGroups++
			if totalGroups > maxOIDCGroups {
				return invalid("role_mappings", fmt.Sprintf("There can be at most %d OIDC groups across all role mappings.", maxOIDCGroups))
			}
		}
	}

	return nil
}

// ValidateBackupOptions checks a service-supplied backup catalog. Services are
// independently developed, so their responses are treated as untrusted input.
func ValidateBackupOptions(options []BackupOption) error {
	if len(options) == 0 {
		return fmt.Errorf("backup options must contain at least one option")
	}
	if len(options) > maxBackupOptions {
		return fmt.Errorf("backup options must contain at most %d options", maxBackupOptions)
	}

	identifiers := make(map[int32]struct{}, len(options))
	names := make(map[string]struct{}, len(options))
	defaults := 0
	for index, option := range options {
		if option.ID < 1 {
			return fmt.Errorf("options[%d].id must be greater than zero", index)
		}
		if _, exists := identifiers[option.ID]; exists {
			return fmt.Errorf("options[%d].id must be unique", index)
		}
		identifiers[option.ID] = struct{}{}

		name := strings.TrimSpace(option.Option)
		if name == "" || name != option.Option || len(option.Option) > maxBackupOptionName {
			return fmt.Errorf("options[%d].option must be between 1 and %d characters without surrounding whitespace", index, maxBackupOptionName)
		}
		if _, exists := names[option.Option]; exists {
			return fmt.Errorf("options[%d].option must be unique", index)
		}
		names[option.Option] = struct{}{}

		if len(option.Description) > maxBackupOptionDescription {
			return fmt.Errorf("options[%d].description must be at most %d characters", index, maxBackupOptionDescription)
		}
		if option.Default {
			defaults++
		}
	}
	if defaults > 1 {
		return fmt.Errorf("backup options must not mark more than one option as the default")
	}

	return nil
}

func ValidateRoleCatalog(roles []RoleDefinition) error {
	if len(roles) > maxServiceRoles {
		return fmt.Errorf("role catalog must contain at most %d roles", maxServiceRoles)
	}

	keys := make(map[string]struct{}, len(roles))
	priorities := make(map[int32]struct{}, len(roles))
	for index, role := range roles {
		if len(role.Key) > maxRoleKeyLength || !roleKeyPattern.MatchString(role.Key) {
			return fmt.Errorf("roles[%d].key must be a stable lowercase role key", index)
		}
		if _, exists := keys[role.Key]; exists {
			return fmt.Errorf("roles[%d].key must be unique", index)
		}
		keys[role.Key] = struct{}{}

		if strings.TrimSpace(role.Name) == "" || role.Name != strings.TrimSpace(role.Name) || len(role.Name) > maxRoleName {
			return fmt.Errorf("roles[%d].name must be between 1 and 128 characters without surrounding whitespace", index)
		}
		if role.Priority < 0 {
			return fmt.Errorf("roles[%d].priority must be zero or greater", index)
		}
		if _, exists := priorities[role.Priority]; exists {
			return fmt.Errorf("roles[%d].priority must be unique", index)
		}
		priorities[role.Priority] = struct{}{}
	}

	return nil
}
