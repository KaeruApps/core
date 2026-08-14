package api

import (
	"context"
	"net/http"

	"github.com/KaeruApps/core/internal/identity"
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

type ServiceIconManager interface {
	Get(context.Context, string) (registry.ServiceIcon, error)
}

type Dependencies struct {
	ApplicationInfo             ApplicationInfo
	ServiceRegistrar            ServiceRegistrar
	ServiceConfigurationManager ServiceConfigurationManager
	ServiceIconManager          ServiceIconManager
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

	router.Route("/api/v1", func(apiRouter chi.Router) {
		// Public API
		apiRouter.Get("/health", health(dependencies.Initialized, dependencies.DevelopmentMode))
		apiRouter.Get("/about", about(dependencies.ApplicationInfo))
		apiRouter.Get("/session", currentSession)
		apiRouter.Get("/services", listServices(dependencies.ServiceConfigurationManager))
		apiRouter.Get("/services/{serviceID}", getService(dependencies.ServiceConfigurationManager))
		apiRouter.Get("/services/{serviceID}/icon", getServiceIcon(dependencies.ServiceIconManager))
		apiRouter.Put("/services/{serviceID}", updateService(dependencies.ServiceConfigurationManager))
		apiRouter.Post("/services/{serviceID}/unregister", unregisterService(dependencies.ServiceConfigurationManager))

		// Internal API
		apiRouter.Route("/internal", func(internalRouter chi.Router) {
			internalRouter.Post("/services/register", registerService(dependencies.ServiceRegistrar))
		})
	})

	return router
}
