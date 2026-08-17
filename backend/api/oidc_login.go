package api

import (
	"errors"
	"net/http"

	"github.com/KaeruApps/core/internal/installation"
)

func startOIDCLogin(manager OIDCLoginManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "oidc_login_unavailable", "OIDC login is unavailable.", "")
			return
		}
		authorization, err := manager.Start(request.Context(), request.Header.Get("Origin"))
		if err == nil {
			writeJSON(response, http.StatusOK, authorization)
			return
		}
		var validationError *installation.ValidationError
		switch {
		case errors.As(err, &validationError):
			writeError(response, http.StatusBadRequest, "validation_failed", validationError.Message, validationError.Field)
		case errors.Is(err, installation.ErrOIDCDiscovery):
			writeError(response, http.StatusBadGateway, "oidc_discovery_failed", "The identity provider could not be reached.", "")
		case errors.Is(err, installation.ErrOIDCAccessURL):
			writeError(response, http.StatusBadRequest, "oidc_access_url_not_allowed", "This URL is not configured for OIDC login.", "")
		default:
			writeError(response, http.StatusInternalServerError, "internal_error", "OIDC login could not be started.", "")
		}
	}
}
