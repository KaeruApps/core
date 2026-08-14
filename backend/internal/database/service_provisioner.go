package database

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"

	"github.com/KaeruApps/core/internal/registry"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxServiceTypeInDatabaseName = 57

var (
	managedDatabaseNamePattern = regexp.MustCompile(`^kaeru_[a-z][a-z0-9_-]{0,56}$`)
	serviceTypePattern         = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

type ServiceDatabaseProvisioner struct {
	pool *pgxpool.Pool
	host string
	port uint16
}

func NewServiceDatabaseProvisioner(pool *pgxpool.Pool, host string, port uint16) *ServiceDatabaseProvisioner {
	return &ServiceDatabaseProvisioner{pool: pool, host: host, port: port}
}

func (provisioner *ServiceDatabaseProvisioner) Provision(ctx context.Context, service registry.Service) (registry.DatabaseCredentials, error) {
	databaseName, err := serviceDatabaseName(service)
	if err != nil {
		return registry.DatabaseCredentials{}, err
	}
	if service.DatabaseName != "" || service.DatabaseUsername != "" {
		if service.DatabaseName != databaseName || service.DatabaseUsername != databaseName {
			return registry.DatabaseCredentials{}, fmt.Errorf("stored database identity does not match service type %q", service.ServiceType)
		}
		return provisioner.reactivate(ctx, databaseName)
	}

	password, err := newDatabasePassword()
	if err != nil {
		return registry.DatabaseCredentials{}, fmt.Errorf("generate service database password: %w", err)
	}

	identifier := pgx.Identifier{databaseName}.Sanitize()
	var createRole string
	err = provisioner.pool.QueryRow(ctx, `
		SELECT format(
			'CREATE ROLE %I WITH LOGIN PASSWORD %L NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION CONNECTION LIMIT 20',
			$1::text,
			$2::text
		)
	`, databaseName, password).Scan(&createRole)
	if err != nil {
		return registry.DatabaseCredentials{}, fmt.Errorf("build service database role statement: %w", err)
	}
	if _, err := provisioner.pool.Exec(ctx, createRole); err != nil {
		return registry.DatabaseCredentials{}, fmt.Errorf("create service database role: %w", err)
	}

	createDatabase := fmt.Sprintf("CREATE DATABASE %s WITH OWNER %s ENCODING 'UTF8' TEMPLATE template0", identifier, identifier)
	if _, err := provisioner.pool.Exec(ctx, createDatabase); err != nil {
		_, dropRoleErr := provisioner.pool.Exec(ctx, "DROP ROLE IF EXISTS "+identifier)
		return registry.DatabaseCredentials{}, errors.Join(
			fmt.Errorf("create service database: %w", err),
			dropRoleErr,
		)
	}

	credentials := registry.DatabaseCredentials{
		Host:     provisioner.host,
		Port:     provisioner.port,
		Database: databaseName,
		Username: databaseName,
		Password: password,
	}
	if err := provisioner.restrictDatabase(ctx, credentials); err != nil {
		return registry.DatabaseCredentials{}, errors.Join(err, provisioner.Deprovision(ctx, credentials))
	}

	return credentials, nil
}

func (provisioner *ServiceDatabaseProvisioner) Suspend(ctx context.Context, credentials registry.DatabaseCredentials) error {
	if err := validateManagedDatabaseCredentials(credentials); err != nil {
		return err
	}

	identifier := pgx.Identifier{credentials.Username}.Sanitize()
	if _, err := provisioner.pool.Exec(ctx, "ALTER ROLE "+identifier+" NOLOGIN"); err != nil {
		return fmt.Errorf("disable service database login: %w", err)
	}
	if _, err := provisioner.pool.Exec(ctx, `
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE usename = $1
		  AND pid <> pg_backend_pid()
	`, credentials.Username); err != nil {
		return fmt.Errorf("terminate service database connections: %w", err)
	}

	return nil
}

func (provisioner *ServiceDatabaseProvisioner) Resume(ctx context.Context, credentials registry.DatabaseCredentials) error {
	if err := validateManagedDatabaseCredentials(credentials); err != nil {
		return err
	}

	identifier := pgx.Identifier{credentials.Username}.Sanitize()
	if _, err := provisioner.pool.Exec(ctx, "ALTER ROLE "+identifier+" LOGIN"); err != nil {
		return fmt.Errorf("restore service database login: %w", err)
	}
	return nil
}

func (provisioner *ServiceDatabaseProvisioner) Deprovision(ctx context.Context, credentials registry.DatabaseCredentials) error {
	if err := validateManagedDatabaseCredentials(credentials); err != nil {
		return err
	}

	identifier := pgx.Identifier{credentials.Database}.Sanitize()
	_, databaseErr := provisioner.pool.Exec(ctx, "DROP DATABASE IF EXISTS "+identifier+" WITH (FORCE)")
	_, roleErr := provisioner.pool.Exec(ctx, "DROP ROLE IF EXISTS "+identifier)
	return errors.Join(databaseErr, roleErr)
}

func (provisioner *ServiceDatabaseProvisioner) reactivate(ctx context.Context, databaseName string) (registry.DatabaseCredentials, error) {
	password, err := newDatabasePassword()
	if err != nil {
		return registry.DatabaseCredentials{}, fmt.Errorf("generate service database password: %w", err)
	}

	var alterRole string
	if err := provisioner.pool.QueryRow(ctx, `
		SELECT format('ALTER ROLE %I WITH LOGIN PASSWORD %L', $1::text, $2::text)
	`, databaseName, password).Scan(&alterRole); err != nil {
		return registry.DatabaseCredentials{}, fmt.Errorf("build service database role reactivation statement: %w", err)
	}
	if _, err := provisioner.pool.Exec(ctx, alterRole); err != nil {
		return registry.DatabaseCredentials{}, fmt.Errorf("reactivate service database role: %w", err)
	}

	credentials := registry.DatabaseCredentials{
		Host:     provisioner.host,
		Port:     provisioner.port,
		Database: databaseName,
		Username: databaseName,
		Password: password,
	}
	if err := provisioner.restrictDatabase(ctx, credentials); err != nil {
		return registry.DatabaseCredentials{}, errors.Join(err, provisioner.Suspend(ctx, credentials))
	}
	return credentials, nil
}

func (provisioner *ServiceDatabaseProvisioner) restrictDatabase(ctx context.Context, credentials registry.DatabaseCredentials) error {
	identifier := pgx.Identifier{credentials.Database}.Sanitize()
	if _, err := provisioner.pool.Exec(ctx, "REVOKE CONNECT ON DATABASE "+identifier+" FROM PUBLIC"); err != nil {
		return fmt.Errorf("restrict service database access: %w", err)
	}
	if _, err := provisioner.pool.Exec(ctx, "GRANT CONNECT ON DATABASE "+identifier+" TO "+identifier); err != nil {
		return fmt.Errorf("grant service database access: %w", err)
	}
	return nil
}

func serviceDatabaseName(service registry.Service) (string, error) {
	serviceType := service.ServiceType
	if !serviceTypePattern.MatchString(serviceType) {
		return "", fmt.Errorf("create service database name: invalid service type %q", serviceType)
	}
	if len(serviceType) > maxServiceTypeInDatabaseName {
		return "", fmt.Errorf("create service database name: service type must be at most %d characters", maxServiceTypeInDatabaseName)
	}
	return "kaeru_" + serviceType, nil
}

func validateManagedDatabaseCredentials(credentials registry.DatabaseCredentials) error {
	if credentials.Database != credentials.Username || !managedDatabaseNamePattern.MatchString(credentials.Database) {
		return fmt.Errorf("refuse to manage unmanaged service database %q", credentials.Database)
	}
	return nil
}

func newDatabasePassword() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}
