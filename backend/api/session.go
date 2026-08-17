package api

import (
	"net/http"
	"time"

	"github.com/KaeruApps/core/internal/identity"
)

func withPrincipal(principal identity.Principal) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			next.ServeHTTP(response, request.WithContext(identity.WithPrincipal(request.Context(), principal)))
		})
	}
}

func withSessionAuthentication(authenticator SessionAuthenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if _, exists := identity.FromContext(request.Context()); exists || authenticator == nil {
				next.ServeHTTP(response, request)
				return
			}
			cookie, err := request.Cookie(sessionCookieName)
			if err != nil {
				next.ServeHTTP(response, request)
				return
			}
			principal, authenticated, err := authenticator.Authenticate(request.Context(), cookie.Value)
			if err != nil || !authenticated {
				next.ServeHTTP(response, request)
				return
			}
			next.ServeHTTP(response, request.WithContext(identity.WithPrincipal(request.Context(), principal)))
		})
	}
}

func requireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if _, authenticated := identity.FromContext(request.Context()); !authenticated {
			writeError(response, http.StatusUnauthorized, "authentication_required", "Authentication is required.", "")
			return
		}
		next.ServeHTTP(response, request)
	})
}

func requireCoreAdministrator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		principal, authenticated := identity.FromContext(request.Context())
		if !authenticated {
			writeError(response, http.StatusUnauthorized, "authentication_required", "Authentication is required.", "")
			return
		}
		if principal.ServiceRoles["core"] != "admin" {
			writeError(response, http.StatusForbidden, "administrator_required", "Kaeru Core administrator access is required.", "")
			return
		}
		next.ServeHTTP(response, request)
	})
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

func logoutSession(manager SessionLogoutManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		cookie, err := request.Cookie(sessionCookieName)
		if err == nil && manager != nil {
			if err := manager.Logout(request.Context(), cookie.Value); err != nil {
				writeError(response, http.StatusInternalServerError, "logout_failed", "The session could not be logged out.", "")
				return
			}
		}
		http.SetCookie(response, &http.Cookie{
			Name:     sessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			Expires:  time.Unix(1, 0),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		response.WriteHeader(http.StatusNoContent)
	}
}
