package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KaeruApps/core/api"
	"github.com/KaeruApps/core/internal/config"
	"github.com/KaeruApps/core/internal/database"
	"github.com/KaeruApps/core/internal/identity"
	"github.com/KaeruApps/core/internal/installation"
	"github.com/KaeruApps/core/internal/registry"
	"github.com/KaeruApps/core/internal/serviceclient"
)

var version = "development"

const applicationDescription = "The shared foundation for the self-hosted Kaeru Platform."

func main() {
	databaseURL := flag.String(
		"database-url",
		"postgresql://kaeru:kaeru_dev@localhost:5432/kaeru?sslmode=disable",
		"PostgreSQL URL used by Kaeru Core",
	)
	serviceDatabaseHost := flag.String(
		"service-database-host",
		"kaeru-postgres",
		"PostgreSQL host returned to registered services",
	)
	serviceDatabasePort := flag.Uint(
		"service-database-port",
		5432,
		"PostgreSQL port returned to registered services",
	)
	coreInternalURL := flag.String(
		"core-internal-url",
		"http://localhost:3000",
		"Internal URL recorded for Kaeru Core",
	)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	applicationContext, stopApplication := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopApplication()
	runtimeConfig, err := config.LoadRuntime()
	if err != nil {
		logger.Error("Kaeru Core configuration is invalid", "error", err)
		os.Exit(1)
	}
	if *serviceDatabasePort > 65535 {
		logger.Error("Service database port is invalid", "port", *serviceDatabasePort)
		os.Exit(1)
	}
	databaseContext, cancelDatabase := context.WithTimeout(applicationContext, 15*time.Second)
	defer cancelDatabase()

	databasePool, err := database.Open(databaseContext, *databaseURL)
	if err != nil {
		logger.Error("Kaeru Core database failed to open", "error", err)
		os.Exit(1)
	}
	defer databasePool.Close()

	serviceStore := database.NewRegistryStore(databasePool)
	installationStore := database.NewInstallationStore(databasePool)
	oidcSetupStore := database.NewOIDCSetupStore(databasePool)
	oidcSetupManager := installation.NewOIDCSetupManager(oidcSetupStore, 10*time.Second)
	oidcLoginManager := installation.NewOIDCLoginManager(oidcSetupStore, 10*time.Second)
	oidcSettingsManager := installation.NewOIDCSettingsManager(oidcSetupStore, 10*time.Second)
	oidcCallbackManager := identity.NewOIDCCallbackManager(oidcSetupStore, 10*time.Second)
	identityStore := database.NewIdentityStore(databasePool)
	sessionManager := identity.NewSessionManager(identityStore)
	userPreferencesStore := database.NewUserPreferencesStore(databasePool)
	userPreferencesManager := identity.NewUserPreferencesManager(userPreferencesStore)
	userAvatarManager := identity.NewUserAvatarManager(userPreferencesStore)
	userDirectory := identity.NewUserDirectory(database.NewUserDirectoryStore(databasePool))
	now := time.Now().UTC()
	if err := serviceStore.EnsureCoreService(
		databaseContext,
		registry.NewCoreService(version, *coreInternalURL, now),
		now,
	); err != nil {
		logger.Error("Kaeru Core service registration failed", "error", err)
		os.Exit(1)
	}
	serviceProvisioner := database.NewServiceDatabaseProvisioner(
		databasePool,
		*serviceDatabaseHost,
		uint16(*serviceDatabasePort),
	)
	iconClient := serviceclient.NewIconClient(2 * time.Second)
	serviceIconManager := registry.NewServiceIconManager(serviceStore, iconClient)
	healthClient := serviceclient.NewHealthClient(2 * time.Second)
	healthMonitor := registry.NewHealthMonitor(serviceStore, healthClient, logger)
	serviceRegistrar := registry.NewRegistrar(serviceStore, serviceProvisioner, serviceIconManager).
		WithHealthRefresher(healthMonitor)
	roleCatalogClient := serviceclient.NewRoleCatalogClient(2 * time.Second)
	serviceManager := registry.NewServiceManager(serviceStore, roleCatalogClient, serviceProvisioner)
	var developmentPrincipal *identity.Principal
	if runtimeConfig.DevelopmentAuth {
		principal := identity.DevelopmentPrincipal()
		developmentPrincipal = &principal
		logger.Warn("Development authentication bypass is enabled; do not expose this server to untrusted networks")
	}
	server := &http.Server{
		Addr: ":3000",
		Handler: api.NewRouter(api.Dependencies{
			ApplicationInfo: api.ApplicationInfo{
				Name:        registry.CoreServiceName,
				Version:     version,
				Description: applicationDescription,
			},
			ServiceRegistrar:            serviceRegistrar,
			ServiceConfigurationManager: serviceManager,
			ServiceIconManager:          serviceIconManager,
			InstallationState:           installationStore,
			OIDCSetupManager:            oidcSetupManager,
			OIDCLoginManager:            oidcLoginManager,
			OIDCSettingsManager:         oidcSettingsManager,
			OIDCCallbackManager:         oidcCallbackManager,
			SessionAuthenticator:        sessionManager,
			SessionLogoutManager:        sessionManager,
			UserPreferencesManager:      userPreferencesManager,
			UserAvatarManager:           userAvatarManager,
			UserDirectory:               userDirectory,
			Initialized:                 runtimeConfig.DevelopmentAuth,
			DevelopmentMode:             runtimeConfig.DevelopmentAuth,
			DevelopmentPrincipal:        developmentPrincipal,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go healthMonitor.Run(applicationContext)
	go func() {
		logger.Info("Kaeru Core listening", "address", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-applicationContext.Done():
		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("Kaeru Core shutdown failed", "error", err)
			return
		}
		if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Kaeru Core stopped unexpectedly", "error", err)
		}
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Kaeru Core stopped unexpectedly", "error", err)
		}
	}
}
