package registry

import "context"

// UnavailableProvisioner keeps the registration API honest until PostgreSQL
// provisioning is implemented. It never returns placeholder credentials.
type UnavailableProvisioner struct{}

func (UnavailableProvisioner) Provision(context.Context, Service) (DatabaseCredentials, error) {
	return DatabaseCredentials{}, ErrProvisioningUnavailable
}

func (UnavailableProvisioner) Deprovision(context.Context, DatabaseCredentials) error {
	return nil
}
