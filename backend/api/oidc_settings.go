package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/KaeruApps/core/internal/installation"
)

func getOIDCSettings(manager OIDCSettingsManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "oidc_settings_unavailable", "OIDC settings are unavailable.", "")
			return
		}
		settings, err := manager.Get(request.Context())
		if err != nil {
			writeError(response, http.StatusInternalServerError, "internal_error", "OIDC settings could not be loaded.", "")
			return
		}
		writeJSON(response, http.StatusOK, settings)
	}
}

func getOIDCBranding(manager OIDCSettingsManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "oidc_branding_unavailable", "OIDC branding is unavailable.", "")
			return
		}
		branding, err := manager.GetBranding(request.Context())
		if err != nil {
			writeError(response, http.StatusServiceUnavailable, "oidc_branding_unavailable", "OIDC branding is unavailable.", "")
			return
		}
		writeJSON(response, http.StatusOK, branding)
	}
}

func getOIDCButtonImage(manager OIDCSettingsManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "oidc_settings_unavailable", "OIDC settings are unavailable.", "")
			return
		}
		image, err := manager.GetButtonImage(request.Context())
		if err != nil || len(image.Content) == 0 {
			writeError(response, http.StatusNotFound, "oidc_button_image_not_found", "No OIDC button image is configured.", "")
			return
		}
		response.Header().Set("Content-Type", image.ContentType)
		response.Header().Set("Cache-Control", "private, max-age=300")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(image.Content)
	}
}

func updateOIDCSettings(manager OIDCSettingsManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "oidc_settings_unavailable", "OIDC settings are unavailable.", "")
			return
		}
		request.Body = http.MaxBytesReader(response, request.Body, maxOIDCSetupBodyBytes)
		if err := request.ParseMultipartForm(maxOIDCSetupBodyBytes); err != nil {
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
		settings, err := manager.Update(request.Context(), installation.OIDCSetupInput{
			Name:                   request.FormValue("name"),
			AccessURLs:             splitCommaOrLineSeparated(request.FormValue("access_urls")),
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
			writeJSON(response, http.StatusOK, settings)
			return
		}
		var validationError *installation.ValidationError
		switch {
		case errors.As(err, &validationError):
			writeError(response, http.StatusBadRequest, "validation_failed", validationError.Message, validationError.Field)
		case errors.Is(err, installation.ErrOIDCDiscovery):
			writeError(response, http.StatusUnprocessableEntity, "oidc_discovery_failed", "The OIDC provider could not be discovered from the issuer URL.", "issuer_url")
		case errors.Is(err, installation.ErrOIDCVerificationRequired):
			writeError(response, http.StatusConflict, "oidc_verification_required", "These OIDC changes must be verified before they can be saved.", "")
		default:
			writeError(response, http.StatusInternalServerError, "internal_error", "OIDC settings could not be saved.", "")
		}
	}
}

func splitCommaOrLineSeparated(value string) []string {
	return strings.FieldsFunc(value, func(character rune) bool {
		return character == ',' || character == '\n' || character == '\r'
	})
}

func verifyOIDCSettings(manager OIDCSettingsManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "oidc_settings_unavailable", "OIDC settings are unavailable.", "")
			return
		}
		request.Body = http.MaxBytesReader(response, request.Body, maxOIDCSetupBodyBytes)
		if err := request.ParseMultipartForm(maxOIDCSetupBodyBytes); err != nil {
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
		authorization, err := manager.StartVerification(request.Context(), installation.OIDCSetupInput{
			Name: request.FormValue("name"), AccessURLs: splitCommaOrLineSeparated(request.FormValue("access_urls")),
			IssuerURL: request.FormValue("issuer_url"), ClientID: request.FormValue("client_id"),
			ClientSecret: request.FormValue("client_secret"), AdditionalScopes: strings.Fields(request.FormValue("additional_scopes")),
			UsernameClaim: request.FormValue("username_claim"), DisplayNameClaim: request.FormValue("display_name_claim"),
			AvatarClaim: request.FormValue("avatar_claim"), GroupsClaim: request.FormValue("groups_claim"),
			AdminGroups: strings.Split(request.FormValue("admin_groups"), ","), ButtonText: request.FormValue("button_text"),
			ButtonImage: image, ButtonImageContentType: imageContentType,
		}, request.Header.Get("Origin"))
		if err == nil {
			http.SetCookie(response, &http.Cookie{Name: oidcSettingsVerificationCookieName, Value: "1", Path: "/api/v1/auth/oidc/callback", MaxAge: 600, HttpOnly: true, Secure: request.TLS != nil, SameSite: http.SameSiteLaxMode})
			writeJSON(response, http.StatusOK, authorization)
			return
		}
		var validationError *installation.ValidationError
		switch {
		case errors.As(err, &validationError):
			writeError(response, http.StatusBadRequest, "validation_failed", validationError.Message, validationError.Field)
		case errors.Is(err, installation.ErrOIDCAccessURL):
			writeError(response, http.StatusBadRequest, "oidc_access_url_not_allowed", "The current URL must remain in Access URLs for verification.", "access_urls")
		case errors.Is(err, installation.ErrOIDCDiscovery):
			writeError(response, http.StatusUnprocessableEntity, "oidc_discovery_failed", "The OIDC provider could not be discovered from the issuer URL.", "issuer_url")
		default:
			writeError(response, http.StatusInternalServerError, "internal_error", "OIDC verification could not be started.", "")
		}
	}
}
