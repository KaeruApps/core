package database

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/KaeruApps/core/internal/database/dbsqlc"
	"github.com/KaeruApps/core/internal/registry"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type RegistryStore struct {
	database transactionStarter
	queries  *dbsqlc.Queries
}

type transactionStarter interface {
	dbsqlc.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

func NewRegistryStore(database transactionStarter) *RegistryStore {
	return &RegistryStore{database: database, queries: dbsqlc.New(database)}
}

func (store *RegistryStore) EnsureCoreService(ctx context.Context, service registry.Service, refreshedAt time.Time) error {
	serviceID, err := store.queries.EnsureCoreService(ctx, dbsqlc.EnsureCoreServiceParams{
		ID:              service.ID,
		ServiceType:     service.ServiceType,
		InstanceID:      service.InstanceID,
		Name:            service.Name,
		Version:         service.Version,
		InternalUrl:     service.InternalURL,
		CreatedAt:       timestamptz(service.CreatedAt),
		HealthCheckedAt: timestamptz(service.LastSeenAt),
	})
	if err != nil {
		return fmt.Errorf("ensure Kaeru Core service: %w", err)
	}
	if err := store.SyncServiceRoles(ctx, serviceID, registry.CoreRoleCatalog(), refreshedAt); err != nil {
		return fmt.Errorf("synchronize Kaeru Core roles: %w", err)
	}
	return nil
}

func (store *RegistryStore) ClaimRegistration(ctx context.Context, service registry.Service, tokenHash [sha256.Size]byte) (registry.RegistrationClaim, error) {
	if _, err := store.queries.GetServiceByInstanceID(ctx, service.InstanceID); err == nil {
		return registry.RegistrationClaim{}, registry.ErrAlreadyRegistered
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return registry.RegistrationClaim{}, fmt.Errorf("check service instance registration: %w", err)
	}

	row, err := store.queries.ClaimServiceRegistration(ctx, dbsqlc.ClaimServiceRegistrationParams{
		ID:               service.ID,
		ServiceType:      service.ServiceType,
		InstanceID:       service.InstanceID,
		Name:             service.Name,
		Version:          service.Version,
		InternalUrl:      service.InternalURL,
		PublicUrl:        service.PublicURL,
		ServiceTokenHash: tokenHash[:],
		CreatedAt:        timestamptz(service.CreatedAt),
		LastSeenAt:       timestamptz(service.LastSeenAt),
	})
	if isInstanceIDConflict(err) {
		return registry.RegistrationClaim{}, registry.ErrAlreadyRegistered
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return registry.RegistrationClaim{}, registry.ErrServiceTypeRegistered
	}
	if err != nil {
		return registry.RegistrationClaim{}, fmt.Errorf("claim service registration: %w", err)
	}

	claimed := registry.Service{
		ID:                 row.ID,
		ServiceType:        row.ServiceType,
		InstanceID:         row.InstanceID,
		Name:               row.Name,
		Version:            row.Version,
		InternalURL:        row.InternalUrl,
		PublicURL:          row.PublicUrl,
		DefaultRoleKey:     row.DefaultRoleKey,
		DatabaseHost:       stringValue(row.DatabaseHost),
		DatabasePort:       uint16Value(row.DatabasePort),
		DatabaseName:       stringValue(row.DatabaseName),
		DatabaseUsername:   stringValue(row.DatabaseUsername),
		ProvisioningStatus: row.ProvisioningStatus,
		RegistrationStatus: row.RegistrationStatus,
		AvailabilityStatus: row.AvailabilityStatus,
		HealthCheckedAt:    timePointer(row.HealthCheckedAt),
		HealthError:        stringValue(row.HealthError),
		HealthFailures:     row.ConsecutiveHealthFailures,
		CreatedAt:          timeValue(row.CreatedAt),
		LastSeenAt:         timeValue(row.LastSeenAt),
	}
	return registry.RegistrationClaim{Service: claimed, Reused: claimed.ID != service.ID}, nil
}

func (store *RegistryStore) ListServiceRoles(ctx context.Context, serviceID string) ([]registry.ServiceRole, error) {
	rows, err := store.queries.ListServiceRoles(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list service roles: %w", err)
	}

	roles := make([]registry.ServiceRole, len(rows))
	for index, row := range rows {
		roles[index] = registry.ServiceRole{
			ServiceID: row.ServiceID,
			Key:       row.RoleKey,
			Name:      row.Name,
			Priority:  row.Priority,
			Active:    row.Active,
		}
	}

	return roles, nil
}

func (store *RegistryStore) ListServices(ctx context.Context) ([]registry.Service, error) {
	rows, err := store.queries.ListServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	services := make([]registry.Service, len(rows))
	for index, row := range rows {
		services[index] = registry.Service{
			ID:                 row.ID,
			ServiceType:        row.ServiceType,
			InstanceID:         row.InstanceID,
			Name:               row.Name,
			Version:            row.Version,
			InternalURL:        row.InternalUrl,
			PublicURL:          row.PublicUrl,
			DefaultRoleKey:     row.DefaultRoleKey,
			DatabaseHost:       stringValue(row.DatabaseHost),
			DatabasePort:       uint16Value(row.DatabasePort),
			DatabaseName:       stringValue(row.DatabaseName),
			DatabaseUsername:   stringValue(row.DatabaseUsername),
			ProvisioningStatus: row.ProvisioningStatus,
			RegistrationStatus: row.RegistrationStatus,
			AvailabilityStatus: row.AvailabilityStatus,
			HealthCheckedAt:    timePointer(row.HealthCheckedAt),
			HealthError:        stringValue(row.HealthError),
			HealthFailures:     row.ConsecutiveHealthFailures,
			CreatedAt:          timeValue(row.CreatedAt),
			LastSeenAt:         timeValue(row.LastSeenAt),
		}
	}

	return services, nil
}

func (store *RegistryStore) GetService(ctx context.Context, serviceID string) (registry.ServiceDetails, error) {
	row, err := store.queries.GetServiceByID(ctx, serviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return registry.ServiceDetails{}, registry.ErrServiceNotFound
	}
	if err != nil {
		return registry.ServiceDetails{}, fmt.Errorf("get service: %w", err)
	}

	roles, err := store.ListServiceRoles(ctx, serviceID)
	if err != nil {
		return registry.ServiceDetails{}, err
	}
	groupRows, err := store.queries.ListServiceRoleGroups(ctx, serviceID)
	if err != nil {
		return registry.ServiceDetails{}, fmt.Errorf("list service role groups: %w", err)
	}

	alternates, err := store.listServiceAlternateURLs(ctx, store.queries, serviceID)
	if err != nil {
		return registry.ServiceDetails{}, err
	}

	mappings := make([]registry.ServiceRoleMapping, 0)
	for _, group := range groupRows {
		if len(mappings) == 0 || mappings[len(mappings)-1].RoleKey != group.RoleKey {
			mappings = append(mappings, registry.ServiceRoleMapping{RoleKey: group.RoleKey, OIDCGroups: []string{}})
		}
		mappings[len(mappings)-1].OIDCGroups = append(mappings[len(mappings)-1].OIDCGroups, group.OidcGroup)
	}

	return registry.ServiceDetails{
		Service: registry.Service{
			ID:                 row.ID,
			ServiceType:        row.ServiceType,
			InstanceID:         row.InstanceID,
			Name:               row.Name,
			Version:            row.Version,
			InternalURL:        row.InternalUrl,
			PublicURL:          row.PublicUrl,
			DefaultRoleKey:     row.DefaultRoleKey,
			DatabaseHost:       stringValue(row.DatabaseHost),
			DatabasePort:       uint16Value(row.DatabasePort),
			DatabaseName:       stringValue(row.DatabaseName),
			DatabaseUsername:   stringValue(row.DatabaseUsername),
			ProvisioningStatus: row.ProvisioningStatus,
			RegistrationStatus: row.RegistrationStatus,
			AvailabilityStatus: row.AvailabilityStatus,
			HealthCheckedAt:    timePointer(row.HealthCheckedAt),
			HealthError:        stringValue(row.HealthError),
			HealthFailures:     row.ConsecutiveHealthFailures,
			CreatedAt:          timeValue(row.CreatedAt),
			LastSeenAt:         timeValue(row.LastSeenAt),
		},
		AlternateURLs: alternates,
		Roles:         roles,
		RoleMappings:  mappings,
		RoleCatalog:   roleCatalogState(row.RoleCatalogRefreshedAt, row.RoleCatalogRefreshError),
	}, nil
}

func (store *RegistryStore) AbandonRegistration(ctx context.Context, serviceID string, reused bool) error {
	if !reused {
		return store.Delete(ctx, serviceID)
	}

	rowsAffected, err := store.queries.AbandonServiceRegistration(ctx, serviceID)
	if err != nil {
		return fmt.Errorf("abandon service registration: %w", err)
	}
	if rowsAffected != 1 {
		return registry.ErrServiceNotFound
	}
	return nil
}

func (store *RegistryStore) UnregisterService(ctx context.Context, serviceID string) error {
	rowsAffected, err := store.queries.UnregisterService(ctx, serviceID)
	if err != nil {
		return fmt.Errorf("unregister service: %w", err)
	}
	if rowsAffected != 1 {
		service, getErr := store.GetService(ctx, serviceID)
		if getErr != nil {
			return getErr
		}
		if service.RegistrationStatus == "unregistered" {
			return nil
		}
		return fmt.Errorf("service cannot be unregistered while status is %q", service.RegistrationStatus)
	}
	return nil
}

func (store *RegistryStore) SyncServiceRoles(ctx context.Context, serviceID string, roles []registry.RoleDefinition, refreshedAt time.Time) error {
	transaction, err := store.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin service role synchronization: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.WithoutCancel(ctx)) }()

	queries := store.queries.WithTx(transaction)
	if err := queries.DeactivateServiceRoles(ctx, serviceID); err != nil {
		return fmt.Errorf("deactivate cached service roles: %w", err)
	}
	for _, role := range roles {
		if err := queries.UpsertServiceRole(ctx, dbsqlc.UpsertServiceRoleParams{
			ServiceID: serviceID,
			RoleKey:   role.Key,
			Name:      role.Name,
			Priority:  role.Priority,
		}); err != nil {
			return fmt.Errorf("synchronize service role %q: %w", role.Key, err)
		}
	}
	rowsAffected, err := queries.CompleteRoleCatalogRefresh(ctx, dbsqlc.CompleteRoleCatalogRefreshParams{
		ID:                     serviceID,
		RoleCatalogRefreshedAt: timestamptz(refreshedAt),
	})
	if err != nil {
		return fmt.Errorf("complete service role synchronization: %w", err)
	}
	if rowsAffected != 1 {
		return registry.ErrServiceNotFound
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit service role synchronization: %w", err)
	}

	return nil
}

func (store *RegistryStore) RecordRoleCatalogFailure(ctx context.Context, serviceID string, message string) error {
	rowsAffected, err := store.queries.RecordRoleCatalogRefreshFailure(ctx, dbsqlc.RecordRoleCatalogRefreshFailureParams{
		ID:                      serviceID,
		RoleCatalogRefreshError: &message,
	})
	if err != nil {
		return fmt.Errorf("record service role catalog failure: %w", err)
	}
	if rowsAffected != 1 {
		return registry.ErrServiceNotFound
	}

	return nil
}

func (store *RegistryStore) UpdateService(ctx context.Context, serviceID string, input registry.UpdateServiceInput) (registry.ServiceDetails, error) {
	transaction, err := store.database.Begin(ctx)
	if err != nil {
		return registry.ServiceDetails{}, fmt.Errorf("begin service configuration update: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.WithoutCancel(ctx)) }()

	queries := store.queries.WithTx(transaction)
	var serviceType string
	if err := transaction.QueryRow(ctx, `SELECT service_type FROM services WHERE id = $1`, serviceID).Scan(&serviceType); errors.Is(err, pgx.ErrNoRows) {
		return registry.ServiceDetails{}, registry.ErrServiceNotFound
	} else if err != nil {
		return registry.ServiceDetails{}, fmt.Errorf("load service type: %w", err)
	}
	rowsAffected, err := queries.UpdateServiceConfiguration(ctx, dbsqlc.UpdateServiceConfigurationParams{
		ID:             serviceID,
		PublicUrl:      input.PublicURL,
		DefaultRoleKey: input.DefaultRoleKey,
	})
	if err != nil {
		return registry.ServiceDetails{}, fmt.Errorf("update service configuration: %w", err)
	}
	if rowsAffected != 1 {
		return registry.ServiceDetails{}, registry.ErrServiceNotFound
	}
	if serviceType == registry.CoreServiceType && input.PublicURL != "" {
		if _, err := transaction.Exec(ctx, `
			UPDATE oidc_settings
			SET access_urls = ARRAY[$1] || array_remove(access_urls, $1)
			WHERE singleton = TRUE
		`, input.PublicURL); err != nil {
			return registry.ServiceDetails{}, fmt.Errorf("synchronize Kaeru Core access URL: %w", err)
		}
	}
	if err := store.applyAlternateURLs(ctx, queries, serviceID, serviceType, input.AlternateURLs); err != nil {
		return registry.ServiceDetails{}, err
	}
	if err := queries.DeleteServiceRoleGroups(ctx, serviceID); err != nil {
		return registry.ServiceDetails{}, fmt.Errorf("replace service role mappings: %w", err)
	}
	for _, mapping := range input.RoleMappings {
		for _, group := range mapping.OIDCGroups {
			if err := queries.CreateServiceRoleGroup(ctx, dbsqlc.CreateServiceRoleGroupParams{
				ServiceID: serviceID,
				RoleKey:   mapping.RoleKey,
				OidcGroup: group,
			}); err != nil {
				return registry.ServiceDetails{}, fmt.Errorf("store service role mapping: %w", err)
			}
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return registry.ServiceDetails{}, fmt.Errorf("commit service configuration update: %w", err)
	}

	return store.GetService(ctx, serviceID)
}

func (store *RegistryStore) CompleteProvisioning(ctx context.Context, serviceID string, credentials registry.DatabaseCredentials) error {
	port := int32(credentials.Port)
	rowsAffected, err := store.queries.CompleteServiceProvisioning(ctx, dbsqlc.CompleteServiceProvisioningParams{
		ID:               serviceID,
		DatabaseHost:     &credentials.Host,
		DatabasePort:     &port,
		DatabaseName:     &credentials.Database,
		DatabaseUsername: &credentials.Username,
	})
	if err != nil {
		return fmt.Errorf("store service database metadata: %w", err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("store service database metadata for %q: %w", serviceID, registry.ErrServiceNotFound)
	}

	return nil
}

func (store *RegistryStore) Delete(ctx context.Context, serviceID string) error {
	if err := store.queries.DeleteService(ctx, serviceID); err != nil {
		return fmt.Errorf("delete service registry record: %w", err)
	}
	return nil
}

func (store *RegistryStore) FindByInstanceID(ctx context.Context, instanceID string) (registry.Service, bool, error) {
	row, err := store.queries.GetServiceByInstanceID(ctx, instanceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return registry.Service{}, false, nil
	}
	if err != nil {
		return registry.Service{}, false, fmt.Errorf("find service by instance ID: %w", err)
	}

	return registry.Service{
		ID:                 row.ID,
		ServiceType:        row.ServiceType,
		InstanceID:         row.InstanceID,
		Name:               row.Name,
		Version:            row.Version,
		InternalURL:        row.InternalUrl,
		PublicURL:          row.PublicUrl,
		DefaultRoleKey:     row.DefaultRoleKey,
		DatabaseHost:       stringValue(row.DatabaseHost),
		DatabasePort:       uint16Value(row.DatabasePort),
		DatabaseName:       stringValue(row.DatabaseName),
		DatabaseUsername:   stringValue(row.DatabaseUsername),
		ProvisioningStatus: row.ProvisioningStatus,
		RegistrationStatus: row.RegistrationStatus,
		AvailabilityStatus: row.AvailabilityStatus,
		HealthCheckedAt:    timePointer(row.HealthCheckedAt),
		HealthError:        stringValue(row.HealthError),
		HealthFailures:     row.ConsecutiveHealthFailures,
		CreatedAt:          timeValue(row.CreatedAt),
		LastSeenAt:         timeValue(row.LastSeenAt),
	}, true, nil
}

func (store *RegistryStore) Authenticate(ctx context.Context, serviceID string, serviceToken string) (bool, error) {
	storedHash, err := store.queries.GetServiceTokenHash(ctx, serviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load service token hash: %w", err)
	}

	candidateHash := sha256.Sum256([]byte(serviceToken))
	return subtle.ConstantTimeCompare(storedHash, candidateHash[:]) == 1, nil
}

func (store *RegistryStore) GetServiceIcon(ctx context.Context, serviceID string) (registry.ServiceIcon, error) {
	row, err := store.queries.GetServiceIcon(ctx, serviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return registry.ServiceIcon{}, registry.ErrServiceIconNotFound
	}
	if err != nil {
		return registry.ServiceIcon{}, fmt.Errorf("get service icon: %w", err)
	}
	if len(row.ContentHash) != sha256.Size {
		return registry.ServiceIcon{}, fmt.Errorf("get service icon: stored content hash has an invalid length")
	}
	var contentHash [sha256.Size]byte
	copy(contentHash[:], row.ContentHash)

	return registry.ServiceIcon{
		ServiceID:   row.ServiceID,
		Content:     row.Content,
		ContentType: row.ContentType,
		ContentHash: contentHash,
		SourceETag:  stringValue(row.SourceEtag),
		FetchedAt:   timeValue(row.FetchedAt),
	}, nil
}

func (store *RegistryStore) UpsertServiceIcon(ctx context.Context, icon registry.ServiceIcon) error {
	var sourceETag *string
	if icon.SourceETag != "" {
		sourceETag = &icon.SourceETag
	}
	if err := store.queries.UpsertServiceIcon(ctx, dbsqlc.UpsertServiceIconParams{
		ServiceID:   icon.ServiceID,
		Content:     icon.Content,
		ContentType: icon.ContentType,
		ContentHash: icon.ContentHash[:],
		SourceEtag:  sourceETag,
		FetchedAt:   timestamptz(icon.FetchedAt),
	}); err != nil {
		return fmt.Errorf("store service icon: %w", err)
	}
	return nil
}

func (store *RegistryStore) PrepareHealthChecks(ctx context.Context) error {
	if err := store.queries.MarkRegisteredServicesHealthUnknown(ctx); err != nil {
		return fmt.Errorf("mark registered service health unknown: %w", err)
	}
	return nil
}

func (store *RegistryStore) ListHealthCheckTargets(ctx context.Context) ([]registry.HealthCheckTarget, error) {
	rows, err := store.queries.ListServicesForHealthCheck(ctx)
	if err != nil {
		return nil, fmt.Errorf("list service health check targets: %w", err)
	}
	targets := make([]registry.HealthCheckTarget, len(rows))
	for index, row := range rows {
		targets[index] = registry.HealthCheckTarget{ServiceID: row.ID, InternalURL: row.InternalUrl}
	}
	return targets, nil
}

func (store *RegistryStore) RecordHealthSuccess(ctx context.Context, serviceID string, checkedAt time.Time) error {
	_, err := store.queries.RecordServiceHealthSuccess(ctx, dbsqlc.RecordServiceHealthSuccessParams{
		ID:              serviceID,
		HealthCheckedAt: timestamptz(checkedAt),
	})
	if err != nil {
		return fmt.Errorf("record service health success: %w", err)
	}
	return nil
}

func (store *RegistryStore) RecordHealthUnavailable(ctx context.Context, serviceID string, checkedAt time.Time) error {
	_, err := store.queries.RecordServiceHealthUnavailable(ctx, dbsqlc.RecordServiceHealthUnavailableParams{
		ID:              serviceID,
		HealthCheckedAt: timestamptz(checkedAt),
	})
	if err != nil {
		return fmt.Errorf("record unavailable service health: %w", err)
	}
	return nil
}

func (store *RegistryStore) RecordHealthFailure(ctx context.Context, serviceID string, checkedAt time.Time, message string, threshold int32) error {
	_, err := store.queries.RecordServiceHealthFailure(ctx, dbsqlc.RecordServiceHealthFailureParams{
		ID:                        serviceID,
		HealthCheckedAt:           timestamptz(checkedAt),
		HealthError:               &message,
		ConsecutiveHealthFailures: threshold,
	})
	if err != nil {
		return fmt.Errorf("record service health failure: %w", err)
	}
	return nil
}

// isUniqueViolation reports a duplicate key, which alternate URL group names
// surface as a conflict rather than an internal error.
func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

func isInstanceIDConflict(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) &&
		postgresError.Code == "23505" &&
		postgresError.ConstraintName == "services_instance_id_unique"
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func timeValue(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func timePointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nullableString(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	return value
}

func roleCatalogState(refreshedAt pgtype.Timestamptz, refreshError *string) registry.RoleCatalogState {
	var refreshedAtValue *time.Time
	if refreshedAt.Valid {
		value := refreshedAt.Time
		refreshedAtValue = &value
	}

	status := "unavailable"
	stale := true
	if refreshError != nil && refreshedAt.Valid {
		status = "cached"
	} else if refreshError == nil && refreshedAt.Valid {
		status = "fresh"
		stale = false
	}

	return registry.RoleCatalogState{
		Status:      status,
		RefreshedAt: refreshedAtValue,
		Stale:       stale,
	}
}

func uint16Value(value *int32) uint16 {
	if value == nil {
		return 0
	}
	return uint16(*value)
}

// applyAlternateURLs writes a service's alternate URLs.
//
// Kaeru Core owns the group list, so a Core update also creates, renames, and
// removes groups to match what was submitted. Any other service may only set
// its own URL against groups that already exist.
func (store *RegistryStore) applyAlternateURLs(
	ctx context.Context,
	queries *dbsqlc.Queries,
	serviceID string,
	serviceType string,
	inputs []registry.AlternateURLInput,
) error {
	if serviceType == registry.CoreServiceType {
		if err := store.reconcileAlternateURLGroups(ctx, queries, inputs); err != nil {
			return err
		}
	}

	if err := queries.DeleteServiceAlternateUrls(ctx, serviceID); err != nil {
		return fmt.Errorf("replace service alternate URLs: %w", err)
	}
	for _, input := range inputs {
		url := strings.TrimSpace(input.URL)
		if url == "" || input.GroupID == 0 {
			continue
		}
		if err := queries.SetServiceAlternateUrl(ctx, dbsqlc.SetServiceAlternateUrlParams{
			ServiceID: serviceID,
			GroupID:   input.GroupID,
			Url:       url,
		}); err != nil {
			return fmt.Errorf("set service alternate URL: %w", err)
		}
	}

	return nil
}

// reconcileAlternateURLGroups makes the stored groups match Core's submission,
// assigning identifiers to newly created groups in place.
func (store *RegistryStore) reconcileAlternateURLGroups(
	ctx context.Context,
	queries *dbsqlc.Queries,
	inputs []registry.AlternateURLInput,
) error {
	now := time.Now().UTC()
	keptIDs := make([]int64, 0, len(inputs))
	for index := range inputs {
		name := strings.TrimSpace(inputs[index].Group)
		if inputs[index].GroupID == 0 {
			groupID, err := queries.CreateAlternateUrlGroup(ctx, dbsqlc.CreateAlternateUrlGroupParams{
				Name:      name,
				CreatedAt: timestamptz(now),
			})
			if err != nil {
				if isUniqueViolation(err) {
					return &registry.ValidationError{Field: "alternate_urls", Message: fmt.Sprintf("alternate URL group %q already exists", name)}
				}
				return fmt.Errorf("create alternate URL group: %w", err)
			}
			inputs[index].GroupID = groupID
		} else if _, err := queries.RenameAlternateUrlGroup(ctx, dbsqlc.RenameAlternateUrlGroupParams{
			ID:   inputs[index].GroupID,
			Name: name,
		}); err != nil {
			if isUniqueViolation(err) {
				return &registry.ValidationError{Field: "alternate_urls", Message: fmt.Sprintf("alternate URL group %q already exists", name)}
			}
			return fmt.Errorf("rename alternate URL group: %w", err)
		}
		keptIDs = append(keptIDs, inputs[index].GroupID)
	}

	// Removing a group removes it from every service, which the foreign key
	// cascade takes care of.
	if len(keptIDs) == 0 {
		if err := queries.DeleteAllAlternateUrlGroups(ctx); err != nil {
			return fmt.Errorf("remove alternate URL groups: %w", err)
		}
		return nil
	}
	if err := queries.DeleteAlternateUrlGroupsExcept(ctx, keptIDs); err != nil {
		return fmt.Errorf("remove alternate URL groups: %w", err)
	}

	return nil
}

// listServiceAlternateURLs returns every group with this service's URL for it,
// leaving the URL empty where the service has not supplied one.
func (store *RegistryStore) listServiceAlternateURLs(ctx context.Context, queries *dbsqlc.Queries, serviceID string) ([]registry.ServiceAlternateURL, error) {
	rows, err := queries.ListServiceAlternateUrls(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list service alternate URLs: %w", err)
	}
	alternates := make([]registry.ServiceAlternateURL, len(rows))
	for index, row := range rows {
		alternates[index] = registry.ServiceAlternateURL{
			GroupID: row.ID,
			Group:   row.Name,
			URL:     stringValue(row.Url),
		}
	}
	return alternates, nil
}
