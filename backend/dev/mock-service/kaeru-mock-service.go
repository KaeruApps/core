package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
)

type serviceRole struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Priority int32  `json:"priority"`
}

type roleCatalog struct {
	Roles []serviceRole `json:"roles"`
}

type backupOption struct {
	ID          int32  `json:"id"`
	Option      string `json:"option"`
	Default     bool   `json:"default"`
	Description string `json:"description"`
}

type backupOptionCatalog struct {
	Options []backupOption `json:"options"`
}

var mockBackupOptions = []backupOption{
	{ID: 1, Option: "Full Backup", Default: true, Description: "Database, configuration, and all stored files."},
	{ID: 2, Option: "Database Only", Description: "Database contents without any stored files."},
	{ID: 3, Option: "Configuration Only", Description: "Service configuration without data."},
}

var mockRoles = []serviceRole{
	{Key: "viewer", Name: "Viewer", Priority: 10},
	{Key: "editor", Name: "Editor", Priority: 50},
	{Key: "admin", Name: "Administrator", Priority: 100},
}

func main() {
	port := flag.Uint("port", 3101, "HTTP port used by the mock Kaeru service")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if *port > 65535 {
		logger.Error("Mock service port is invalid", "port", *port)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":" + strconv.FormatUint(uint64(*port), 10),
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	serverStopped := make(chan error, 1)
	go func() {
		logger.Info("Kaeru mock service listening", "address", server.Addr)
		serverStopped <- server.ListenAndServe()
	}()

	select {
	case <-shutdownContext.Done():
		shutdownTimeout, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		if err := server.Shutdown(shutdownTimeout); err != nil {
			logger.Error("Kaeru mock service failed to shut down", "error", err)
			os.Exit(1)
		}
		if err := <-serverStopped; err != nil && err != http.ErrServerClosed {
			logger.Error("Kaeru mock service stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case err := <-serverStopped:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("Kaeru mock service stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}
}

func newHandler() http.Handler {
	router := http.NewServeMux()
	var available atomic.Bool
	available.Store(true)
	router.HandleFunc("GET /api/core/v1/health", func(response http.ResponseWriter, _ *http.Request) {
		writeJSON(response, http.StatusOK, struct {
			Available bool `json:"available"`
		}{Available: available.Load()})
	})
	router.HandleFunc("PUT /api/v1/dev/availability", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Available *bool `json:"available"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || input.Available == nil {
			http.Error(response, "available is required", http.StatusBadRequest)
			return
		}
		available.Store(*input.Available)
		writeJSON(response, http.StatusOK, struct {
			Available bool `json:"available"`
		}{Available: available.Load()})
	})
	router.HandleFunc("GET /api/core/v1/system/roles", func(response http.ResponseWriter, _ *http.Request) {
		writeJSON(response, http.StatusOK, roleCatalog{Roles: mockRoles})
	})
	router.HandleFunc("GET /api/core/v1/system/icon", func(response http.ResponseWriter, _ *http.Request) {
		icon, err := loadKaeruIcon()
		if err != nil {
			http.Error(response, "Kaeru development icon is unavailable", http.StatusInternalServerError)
			return
		}
		response.Header().Set("Content-Type", "image/svg+xml")
		response.Header().Set("ETag", `"kaeru-development-icon"`)
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(icon)
	})

	router.HandleFunc("GET /api/core/v1/backup/options", func(response http.ResponseWriter, _ *http.Request) {
		writeJSON(response, http.StatusOK, backupOptionCatalog{Options: mockBackupOptions})
	})
	router.HandleFunc("GET /api/core/v1/backup", func(response http.ResponseWriter, request *http.Request) {
		option, found := findBackupOption(request.URL.Query().Get("option"))
		if !found {
			http.Error(response, "option must identify a published backup option", http.StatusBadRequest)
			return
		}
		archive := fmt.Sprintf("kaeru-mock-service backup\noption: %s\n", option.Option)
		response.Header().Set("Content-Type", "application/gzip")
		response.Header().Set("Content-Disposition", `attachment; filename="kaeru-mock-service-backup.tar.gz"`)
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte(archive))
	})
	router.HandleFunc("POST /api/core/v1/backup", func(response http.ResponseWriter, request *http.Request) {
		option, found := findBackupOption(request.URL.Query().Get("option"))
		if !found {
			http.Error(response, "option must identify a published backup option", http.StatusBadRequest)
			return
		}
		writeJSON(response, http.StatusAccepted, struct {
			Status string `json:"status"`
			Option string `json:"option"`
		}{Status: "started", Option: option.Option})
	})

	return router
}

// findBackupOption resolves the option query parameter to a published option.
func findBackupOption(raw string) (backupOption, bool) {
	identifier, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return backupOption{}, false
	}
	for _, option := range mockBackupOptions {
		if option.ID == int32(identifier) {
			return option, true
		}
	}
	return backupOption{}, false
}

func loadKaeruIcon() ([]byte, error) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("resolve mock service source path")
	}
	return os.ReadFile(filepath.Join(filepath.Dir(sourceFile), "../../../frontend/src/assets/app-icon.svg"))
}

func writeJSON(response http.ResponseWriter, status int, body any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(body)
}
