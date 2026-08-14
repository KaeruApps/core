package api

import (
	"net/http"

	"github.com/KaeruApps/core/internal/identity"
)

func withPrincipal(principal identity.Principal) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			next.ServeHTTP(response, request.WithContext(identity.WithPrincipal(request.Context(), principal)))
		})
	}
}

func currentSession(response http.ResponseWriter, request *http.Request) {
	principal, authenticated := identity.FromContext(request.Context())
	if !authenticated {
		writeError(response, http.StatusUnauthorized, "authentication_required", "Authentication is required.", "")
		return
	}

	writeJSON(response, http.StatusOK, struct {
		Authenticated bool               `json:"authenticated"`
		User          identity.Principal `json:"user"`
	}{
		Authenticated: true,
		User:          principal,
	})
}
