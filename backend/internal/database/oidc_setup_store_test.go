package database

import (
	"testing"

	"github.com/KaeruApps/core/internal/installation"
)

func TestOIDCAdministratorOnlyChangeKeepsExistingSessions(t *testing.T) {
	current := installation.OIDCConfiguration{OIDCSetupInput: installation.OIDCSetupInput{
		IssuerURL: "https://identity.example.com", ClientID: "kaeru-core", ClientSecret: "secret",
		AdditionalScopes: []string{"groups"}, UsernameClaim: "preferred_username",
		GroupsClaim: "groups", AccessURLs: []string{"https://core.example.com"},
		AdminGroups: []string{"old-admins"},
	}}
	pending := current
	pending.AdminGroups = []string{"new-admins"}

	if oidcAuthenticationRequiresSessionRevocation(current, pending) {
		t.Fatal("administrator-only change should not revoke existing sessions")
	}

	pending.IssuerURL = "https://new-identity.example.com"
	if !oidcAuthenticationRequiresSessionRevocation(current, pending) {
		t.Fatal("identity provider change should revoke existing sessions")
	}
}
