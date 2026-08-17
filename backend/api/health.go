package api

import (
	"net/http"

	"github.com/KaeruApps/core/internal/installation"
)

func health(reader installation.StateReader, initializedOverride bool, developmentMode bool) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		state, err := effectiveInstallationState(request.Context(), reader, initializedOverride)
		initialized := err == nil && state == installation.StateReady
		writeJSON(response, http.StatusOK, struct {
			Available       bool `json:"available"`
			Initialized     bool `json:"initialized"`
			DevelopmentMode bool `json:"development_mode"`
		}{
			Available:       true,
			Initialized:     initialized,
			DevelopmentMode: developmentMode,
		})
	}
}
