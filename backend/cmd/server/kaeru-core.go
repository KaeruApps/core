package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/KaeruApps/core/internal/api"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &http.Server{
		Addr:              ":3000",
		Handler:           api.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("Kaeru Core listening", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		logger.Error("HTTP server stopped", "error", err)
		os.Exit(1)
	}
}
