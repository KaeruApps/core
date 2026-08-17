package api

import "github.com/KaeruApps/core/internal/identity"

func authenticatedTestDependencies(dependencies Dependencies) Dependencies {
	principal := identity.DevelopmentPrincipal()
	dependencies.DevelopmentPrincipal = &principal
	return dependencies
}
