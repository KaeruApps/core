package identity

import (
	"context"
	"testing"
)

func TestDevelopmentPrincipalHasCoreAdministratorAccess(t *testing.T) {
	principal := DevelopmentPrincipal()
	if principal.ServiceRoles["core"] != "admin" {
		t.Fatalf("development principal roles = %#v", principal.ServiceRoles)
	}

	stored, exists := FromContext(WithPrincipal(context.Background(), principal))
	if !exists || stored.Subject != principal.Subject {
		t.Fatalf("principal was not retained in context: %#v", stored)
	}
}
