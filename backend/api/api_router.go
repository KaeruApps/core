package api

import (
	"context"
	"net/http"

	"github.com/KaeruApps/core/internal/identity"
	"github.com/KaeruApps/core/internal/installation"
	"github.com/KaeruApps/core/internal/registry"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type ServiceRegistrar interface {
	Register(context.Context, registry.RegistrationInput) (registry.RegistrationResult, error)
}

type ServiceConfigurationManager interface {
	List(context.Context) ([]registry.Service, error)
	Get(context.Context, string) (registry.ServiceDetails, error)
	Update(context.Context, string, registry.UpdateServiceInput) (registry.ServiceDetails, error)
	Unregister(context.Context, string) (registry.ServiceDetails, error)
}

type BackupOptionsDirectory interface {
	List(context.Context) ([]registry.ServiceBackupOptions, error)
}

type ServiceIconManager interface {
	Get(context.Context, string) (registry.ServiceIcon, error)
}

type OIDCSetupManager interface {
	Start(context.Context, installation.OIDCSetupInput) (installation.OIDCAuthorization, error)
}

type OIDCLoginManager interface {
	Start(context.Context, string) (installation.OIDCAuthorization, error)
}

type OIDCSettingsManager interface {
	Get(context.Context) (installation.OIDCSettings, error)
	GetBranding(context.Context) (installation.OIDCBranding, error)
	GetButtonImage(context.Context) (installation.OIDCButtonImage, error)
	Update(context.Context, installation.OIDCSetupInput) (installation.OIDCSettings, error)
	StartVerification(context.Context, installation.OIDCSetupInput, string) (installation.OIDCAuthorization, error)
}

type OIDCCallbackManager interface {
	Complete(context.Context, string, string, string, string) (identity.OIDCCallbackResult, error)
}

type SessionAuthenticator interface {
	Authenticate(context.Context, string) (identity.Principal, bool, error)
}

type SessionLogoutManager interface {
	Logout(context.Context, string) error
}

type UserPreferencesManager interface {
	Get(context.Context, string) (identity.UserPreferences, error)
	Update(context.Context, string, identity.UserPreferences) (identity.UserPreferences, error)
}

type UserAvatarManager interface {
	Get(context.Context, string) (identity.UserAvatar, error)
	Update(context.Context, string, []byte) (identity.UserAvatar, error)
}

type UserDirectory interface {
	List(context.Context) ([]identity.UserSummary, error)
}

type Dependencies struct {
	ApplicationInfo             ApplicationInfo
	ServiceRegistrar            ServiceRegistrar
	ServiceConfigurationManager ServiceConfigurationManager
	ServiceIconManager          ServiceIconManager
	BackupOptionsDirectory      BackupOptionsDirectory
	InstallationState           installation.StateReader
	OIDCSetupManager            OIDCSetupManager
	OIDCLoginManager            OIDCLoginManager
	OIDCSettingsManager         OIDCSettingsManager
	OIDCCallbackManager         OIDCCallbackManager
	SessionAuthenticator        SessionAuthenticator
	SessionLogoutManager        SessionLogoutManager
	UserPreferencesManager      UserPreferencesManager
	UserAvatarManager           UserAvatarManager
	UserDirectory               UserDirectory
	Initialized                 bool
	DevelopmentMode             bool
	DevelopmentPrincipal        *identity.Principal
}

func NewRouter(dependencies Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	if dependencies.DevelopmentPrincipal != nil {
		router.Use(withPrincipal(*dependencies.DevelopmentPrincipal))
	}
	router.Use(withSessionAuthentication(dependencies.SessionAuthenticator))

	router.Route("/api/v1", func(apiRouter chi.Router) {
		// Public API
		apiRouter.Get("/health", health(dependencies.InstallationState, dependencies.Initialized, dependencies.DevelopmentMode))
		apiRouter.Get("/setup/status", setupStatus(dependencies.InstallationState, dependencies.Initialized, dependencies.DevelopmentMode))
		apiRouter.Post("/setup/oidc", configureOIDC(
			dependencies.OIDCSetupManager,
			dependencies.InstallationState,
			dependencies.Initialized,
		))
		apiRouter.Get("/auth/oidc/callback", completeOIDCCallback(
			dependencies.OIDCCallbackManager,
			dependencies.InstallationState,
			dependencies.Initialized,
		))
		apiRouter.Post("/auth/oidc/login", startOIDCLogin(dependencies.OIDCLoginManager))
		apiRouter.Get("/auth/oidc/branding", getOIDCBranding(dependencies.OIDCSettingsManager))
		apiRouter.Get("/auth/oidc/button-image", getOIDCButtonImage(dependencies.OIDCSettingsManager))
		apiRouter.Get("/about", about(dependencies.ApplicationInfo))
		apiRouter.Group(func(initializedRouter chi.Router) {
			initializedRouter.Use(requireInitialized(dependencies.InstallationState, dependencies.Initialized))
			initializedRouter.Get("/session", currentSession)
			initializedRouter.Post("/session/logout", logoutSession(dependencies.SessionLogoutManager))
			initializedRouter.Group(func(authenticatedRouter chi.Router) {
				authenticatedRouter.Use(requireAuthentication)
				authenticatedRouter.Get("/users/me/preferences", getUserPreferences(dependencies.UserPreferencesManager))
				authenticatedRouter.Put("/users/me/preferences", updateUserPreferences(dependencies.UserPreferencesManager))
				authenticatedRouter.Get("/users/me/avatar", getUserAvatar(dependencies.UserAvatarManager))
				authenticatedRouter.Put("/users/me/avatar", updateUserAvatar(dependencies.UserAvatarManager))
				authenticatedRouter.Group(func(administratorRouter chi.Router) {
					administratorRouter.Use(requireCoreAdministrator)
					administratorRouter.Get("/users", listUsers(dependencies.UserDirectory))
					administratorRouter.Get("/users/{userID}/avatar", getManagedUserAvatar(dependencies.UserAvatarManager))
					administratorRouter.Get("/oidc/settings", getOIDCSettings(dependencies.OIDCSettingsManager))
					administratorRouter.Get("/oidc/settings/button-image", getOIDCButtonImage(dependencies.OIDCSettingsManager))
					administratorRouter.Put("/oidc/settings", updateOIDCSettings(dependencies.OIDCSettingsManager))
					administratorRouter.Post("/oidc/settings/verify", verifyOIDCSettings(dependencies.OIDCSettingsManager))
					administratorRouter.Get("/backup/options", listBackupOptions(dependencies.BackupOptionsDirectory))
					administratorRouter.Get("/services", listServices(dependencies.ServiceConfigurationManager))
					administratorRouter.Get("/services/{serviceID}", getService(dependencies.ServiceConfigurationManager))
					administratorRouter.Get("/services/{serviceID}/icon", getServiceIcon(dependencies.ServiceIconManager))
					administratorRouter.Put("/services/{serviceID}", updateService(dependencies.ServiceConfigurationManager))
					administratorRouter.Post("/services/{serviceID}/unregister", unregisterService(dependencies.ServiceConfigurationManager))
				})
			})
		})

		// Internal API
		apiRouter.Route("/internal", func(internalRouter chi.Router) {
			internalRouter.Post("/services/register", registerService(
				dependencies.ServiceRegistrar,
				dependencies.InstallationState,
				dependencies.Initialized,
			))
		})
	})

	return router
}
