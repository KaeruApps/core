package api

import (
	"errors"
	"net/http"

	"github.com/KaeruApps/core/internal/identity"
)

func getUserPreferences(manager UserPreferencesManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		principal, authenticated := identity.FromContext(request.Context())
		if !authenticated || principal.ID == "" {
			writeError(response, http.StatusUnauthorized, "authentication_required", "Authentication is required.", "")
			return
		}
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "preferences_unavailable", "User preferences are unavailable.", "")
			return
		}
		preferences, err := manager.Get(request.Context(), principal.ID)
		if err == nil {
			writeJSON(response, http.StatusOK, preferences)
			return
		}
		if errors.Is(err, identity.ErrUserNotFound) {
			writeError(response, http.StatusNotFound, "user_not_found", "The current user could not be found.", "")
			return
		}
		writeError(response, http.StatusInternalServerError, "internal_error", "User preferences could not be loaded.", "")
	}
}

func updateUserPreferences(manager UserPreferencesManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		principal, authenticated := identity.FromContext(request.Context())
		if !authenticated || principal.ID == "" {
			writeError(response, http.StatusUnauthorized, "authentication_required", "Authentication is required.", "")
			return
		}
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "preferences_unavailable", "User preferences are unavailable.", "")
			return
		}
		var input identity.UserPreferences
		if err := decodeJSON(response, request, &input); err != nil {
			writeError(response, http.StatusBadRequest, "invalid_request", "The request body must contain one valid JSON object.", "")
			return
		}
		preferences, err := manager.Update(request.Context(), principal.ID, input)
		if err == nil {
			writeJSON(response, http.StatusOK, preferences)
			return
		}
		var validationError *identity.PreferencesValidationError
		switch {
		case errors.As(err, &validationError):
			writeError(response, http.StatusBadRequest, "validation_failed", validationError.Message, validationError.Field)
		case errors.Is(err, identity.ErrUserNotFound):
			writeError(response, http.StatusNotFound, "user_not_found", "The current user could not be found.", "")
		case errors.Is(err, identity.ErrUsernameTaken):
			writeError(response, http.StatusConflict, "username_taken", "That username is already in use.", "username")
		default:
			writeError(response, http.StatusInternalServerError, "internal_error", "User preferences could not be saved.", "")
		}
	}
}
