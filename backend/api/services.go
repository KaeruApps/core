package api

import (
	"errors"
	"net/http"

	"github.com/KaeruApps/core/internal/registry"
	"github.com/go-chi/chi/v5"
)

func listServices(manager ServiceConfigurationManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "service_registry_unavailable", "The service registry is unavailable.", "")
			return
		}

		services, err := manager.List(request.Context())
		if err != nil {
			writeError(response, http.StatusInternalServerError, "internal_error", "Services could not be loaded.", "")
			return
		}

		writeJSON(response, http.StatusOK, services)
	}
}

func getService(manager ServiceConfigurationManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "service_registry_unavailable", "The service registry is unavailable.", "")
			return
		}

		service, err := manager.Get(request.Context(), chi.URLParam(request, "serviceID"))
		if err == nil {
			writeJSON(response, http.StatusOK, service)
			return
		}
		if errors.Is(err, registry.ErrServiceNotFound) {
			writeError(response, http.StatusNotFound, "service_not_found", "The requested service was not found.", "service_id")
			return
		}
		writeError(response, http.StatusInternalServerError, "internal_error", "The service could not be loaded.", "")
	}
}

func updateService(manager ServiceConfigurationManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "service_registry_unavailable", "The service registry is unavailable.", "")
			return
		}

		var input registry.UpdateServiceInput
		if err := decodeJSON(response, request, &input); err != nil {
			writeError(response, http.StatusBadRequest, "invalid_request", "The request body must contain one valid JSON object.", "")
			return
		}

		service, err := manager.Update(request.Context(), chi.URLParam(request, "serviceID"), input)
		if err == nil {
			writeJSON(response, http.StatusOK, service)
			return
		}

		var validationError *registry.ValidationError
		switch {
		case errors.As(err, &validationError):
			writeError(response, http.StatusBadRequest, "validation_failed", validationError.Message, validationError.Field)
		case errors.Is(err, registry.ErrServiceNotFound):
			writeError(response, http.StatusNotFound, "service_not_found", "The requested service was not found.", "service_id")
		case errors.Is(err, registry.ErrCoreAdminVerificationRequired):
			writeError(response, http.StatusConflict, "oidc_verification_required", "Kaeru Core administrator groups must be verified before they can be changed.", "role_mappings")
		default:
			writeError(response, http.StatusInternalServerError, "internal_error", "The service could not be updated.", "")
		}
	}
}

func unregisterService(manager ServiceConfigurationManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "service_registry_unavailable", "The service registry is unavailable.", "")
			return
		}

		service, err := manager.Unregister(request.Context(), chi.URLParam(request, "serviceID"))
		if err == nil {
			writeJSON(response, http.StatusOK, service)
			return
		}
		if errors.Is(err, registry.ErrServiceNotFound) {
			writeError(response, http.StatusNotFound, "service_not_found", "The requested service was not found.", "service_id")
			return
		}
		if errors.Is(err, registry.ErrBuiltInService) {
			writeError(response, http.StatusConflict, "built_in_service", "Kaeru Core cannot be unregistered.", "service_id")
			return
		}

		writeError(response, http.StatusInternalServerError, "internal_error", "The service could not be unregistered.", "")
	}
}
