package api

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/KaeruApps/core/internal/installation"
)

const maxOIDCSetupBodyBytes = 2 * 1024 * 1024

func configureOIDC(
	manager OIDCSetupManager,
	stateReader installation.StateReader,
	initializedOverride bool,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		state, err := effectiveInstallationState(request.Context(), stateReader, initializedOverride)
		if err != nil {
			writeError(response, http.StatusServiceUnavailable, "installation_state_unavailable", "The installation state could not be loaded.", "")
			return
		}
		if state != installation.StateRequired && state != installation.StateConfiguring {
			writeError(response, http.StatusConflict, "setup_already_completed", "OIDC setup cannot be changed in the current installation state.", "")
			return
		}
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "oidc_setup_unavailable", "OIDC setup is unavailable.", "")
			return
		}

		request.Body = http.MaxBytesReader(response, request.Body, maxOIDCSetupBodyBytes)
		if err := request.ParseMultipartForm(maxOIDCSetupBodyBytes); err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				writeError(response, http.StatusRequestEntityTooLarge, "request_too_large", "The OIDC setup request must be 2 MB or smaller.", "")
				return
			}
			writeError(response, http.StatusBadRequest, "invalid_request", "The request must contain valid multipart form data.", "")
			return
		}
		if request.MultipartForm != nil {
			defer request.MultipartForm.RemoveAll()
		}

		image, imageContentType, err := readOptionalButtonImage(request)
		if err != nil {
			writeError(response, http.StatusBadRequest, "validation_failed", err.Error(), "button_image")
			return
		}

		result, err := manager.Start(request.Context(), installation.OIDCSetupInput{
			Name:                   request.FormValue("name"),
			PublicURL:              request.FormValue("public_url"),
			IssuerURL:              request.FormValue("issuer_url"),
			ClientID:               request.FormValue("client_id"),
			ClientSecret:           request.FormValue("client_secret"),
			AdditionalScopes:       strings.Fields(request.FormValue("additional_scopes")),
			UsernameClaim:          request.FormValue("username_claim"),
			DisplayNameClaim:       request.FormValue("display_name_claim"),
			AvatarClaim:            request.FormValue("avatar_claim"),
			GroupsClaim:            request.FormValue("groups_claim"),
			AdminGroups:            strings.Split(request.FormValue("admin_groups"), ","),
			ButtonText:             request.FormValue("button_text"),
			ButtonImage:            image,
			ButtonImageContentType: imageContentType,
		})
		if err == nil {
			writeJSON(response, http.StatusOK, result)
			return
		}

		var validationError *installation.ValidationError
		switch {
		case errors.As(err, &validationError):
			writeError(response, http.StatusBadRequest, "validation_failed", validationError.Message, validationError.Field)
		case errors.Is(err, installation.ErrOIDCDiscovery):
			writeError(response, http.StatusUnprocessableEntity, "oidc_discovery_failed", "The OIDC provider could not be discovered from the issuer URL.", "issuer_url")
		case errors.Is(err, installation.ErrAlreadyInitialized):
			writeError(response, http.StatusConflict, "setup_already_completed", "Kaeru Core setup has already been completed.", "")
		default:
			writeError(response, http.StatusInternalServerError, "internal_error", "The OIDC configuration could not be saved.", "")
		}
	}
}

func readOptionalButtonImage(request *http.Request) ([]byte, string, error) {
	file, header, err := request.FormFile("button_image")
	if errors.Is(err, http.ErrMissingFile) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", errors.New("button image could not be read")
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil {
		return nil, "", errors.New("button image could not be read")
	}
	if len(content) > 1024*1024 {
		return nil, "", errors.New("button image must be 1 MB or smaller")
	}
	return content, header.Header.Get("Content-Type"), nil
}
