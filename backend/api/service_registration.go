package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/KaeruApps/core/internal/installation"
	"github.com/KaeruApps/core/internal/registry"
)

const maxJSONBodyBytes = 64 * 1024

type apiError struct {
	Error apiErrorDetails `json:"error"`
}

type apiErrorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func registerService(
	registrar ServiceRegistrar,
	installationState installation.StateReader,
	initializedOverride bool,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		state, err := effectiveInstallationState(request.Context(), installationState, initializedOverride)
		if err != nil {
			writeError(response, http.StatusServiceUnavailable, "installation_state_unavailable", "The installation state could not be loaded. Retry registration later.", "")
			return
		}
		if state != installation.StateReady {
			response.Header().Set("Retry-After", "15")
			writeError(response, http.StatusServiceUnavailable, "core_not_initialized", "Kaeru Core setup is not complete. Retry registration after Core has been initialized.", "")
			return
		}

		if registrar == nil {
			writeError(response, http.StatusServiceUnavailable, "service_registry_unavailable", "Service registration is unavailable.", "")
			return
		}

		var input registry.RegistrationInput
		if err := decodeJSON(response, request, &input); err != nil {
			writeError(response, http.StatusBadRequest, "invalid_request", "The request body must contain one valid JSON object.", "")
			return
		}

		result, err := registrar.Register(request.Context(), input)
		if err == nil {
			writeJSON(response, http.StatusCreated, result)
			return
		}

		var validationError *registry.ValidationError
		switch {
		case errors.As(err, &validationError):
			writeError(response, http.StatusBadRequest, "validation_failed", validationError.Message, validationError.Field)
		case errors.Is(err, registry.ErrAlreadyRegistered):
			writeError(response, http.StatusConflict, "service_already_registered", "This service instance is already registered.", "instance_id")
		case errors.Is(err, registry.ErrServiceTypeRegistered):
			writeError(response, http.StatusConflict, "service_type_already_registered", "A service of this type is already registered.", "service_type")
		case errors.Is(err, registry.ErrProvisioningUnavailable):
			writeError(response, http.StatusServiceUnavailable, "database_provisioning_unavailable", "Service database provisioning is not available yet.", "")
		default:
			writeError(response, http.StatusInternalServerError, "internal_error", "The service could not be registered.", "")
		}
	}
}

func decodeJSON(response http.ResponseWriter, request *http.Request, destination any) error {
	request.Body = http.MaxBytesReader(response, request.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain a single JSON object")
	}

	return nil
}

func writeError(response http.ResponseWriter, status int, code string, message string, field string) {
	writeJSON(response, status, apiError{
		Error: apiErrorDetails{
			Code:    code,
			Message: message,
			Field:   field,
		},
	})
}

func writeJSON(response http.ResponseWriter, status int, body any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(body)
}
