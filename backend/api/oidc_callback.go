package api

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/KaeruApps/core/internal/identity"
	"github.com/KaeruApps/core/internal/installation"
)

const sessionCookieName = "kaeru_session"
const oidcSettingsVerificationCookieName = "kaeru_oidc_settings_verification"

func completeOIDCCallback(manager OIDCCallbackManager, stateReader installation.StateReader, initializedOverride bool) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		_, verificationCookieErr := request.Cookie(oidcSettingsVerificationCookieName)
		settingsVerification := verificationCookieErr == nil
		if settingsVerification {
			http.SetCookie(response, &http.Cookie{Name: oidcSettingsVerificationCookieName, Value: "", Path: "/api/v1/auth/oidc/callback", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		}
		if providerError := request.URL.Query().Get("error"); providerError != "" {
			redirectOIDCError(response, request, stateReader, initializedOverride, settingsVerification, "The identity provider did not authorize the login.")
			return
		}
		if manager == nil {
			redirectOIDCError(response, request, stateReader, initializedOverride, settingsVerification, "OIDC authentication is unavailable.")
			return
		}

		result, err := manager.Complete(
			request.Context(),
			request.URL.Query().Get("state"),
			request.URL.Query().Get("code"),
			request.UserAgent(),
			requestIPAddress(request),
		)
		if err != nil {
			var claimError *identity.OIDCClaimError
			switch {
			case errors.Is(err, identity.ErrLoginAttemptInvalid):
				redirectOIDCError(response, request, stateReader, initializedOverride, settingsVerification, "This login attempt is invalid or has expired. Please try again.")
			case errors.Is(err, identity.ErrAdminGroupRequired):
				redirectOIDCError(response, request, stateReader, initializedOverride, settingsVerification, "Your account does not belong to the configured administrator group.")
			case errors.Is(err, identity.ErrUserDisabled):
				redirectOIDCError(response, request, stateReader, initializedOverride, settingsVerification, "Your account has been disabled.")
			case errors.As(err, &claimError):
				redirectOIDCError(response, request, stateReader, initializedOverride, settingsVerification, claimError.UserMessage())
			case errors.Is(err, identity.ErrOIDCClaimsInvalid):
				redirectOIDCError(response, request, stateReader, initializedOverride, settingsVerification, "The identity provider did not return the required user claims.")
			default:
				redirectOIDCError(response, request, stateReader, initializedOverride, settingsVerification, "OIDC login could not be completed. Please try again.")
			}
			return
		}

		http.SetCookie(response, &http.Cookie{
			Name:     sessionCookieName,
			Value:    result.SessionToken,
			Path:     "/",
			Expires:  result.ExpiresAt,
			MaxAge:   int(time.Until(result.ExpiresAt).Seconds()),
			HttpOnly: true,
			Secure:   result.SecureCookie,
			SameSite: http.SameSiteLaxMode,
		})
		path := "/"
		if result.Purpose == installation.OIDCLoginPurposeSettingsVerification {
			path = "/?oidc_verification=success"
		}
		http.Redirect(response, request, path, http.StatusSeeOther)
	}
}

func redirectOIDCError(response http.ResponseWriter, request *http.Request, stateReader installation.StateReader, initializedOverride bool, settingsVerification bool, message string) {
	if settingsVerification {
		http.Redirect(response, request, "/?oidc_verification=failed&error="+url.QueryEscape(message), http.StatusSeeOther)
		return
	}
	path := "/setup/oidc"
	if state, err := effectiveInstallationState(request.Context(), stateReader, initializedOverride); err == nil && state == installation.StateReady {
		path = "/login"
	}
	http.Redirect(
		response,
		request,
		path+"?error="+url.QueryEscape(message),
		http.StatusSeeOther,
	)
}

func requestIPAddress(request *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(request.RemoteAddr)
}
