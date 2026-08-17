package api

import (
	"context"
	"net/http"

	"github.com/KaeruApps/core/internal/installation"
)

func effectiveInstallationState(
	ctx context.Context,
	reader installation.StateReader,
	initializedOverride bool,
) (installation.State, error) {
	if initializedOverride {
		return installation.StateReady, nil
	}
	if reader == nil {
		return installation.StateRequired, nil
	}
	return reader.State(ctx)
}

func setupStatus(
	reader installation.StateReader,
	initializedOverride bool,
	developmentMode bool,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		state, err := effectiveInstallationState(request.Context(), reader, initializedOverride)
		if err != nil {
			writeError(response, http.StatusServiceUnavailable, "installation_state_unavailable", "The installation state could not be loaded.", "")
			return
		}

		writeJSON(response, http.StatusOK, struct {
			State           installation.State `json:"state"`
			Initialized     bool               `json:"initialized"`
			DevelopmentMode bool               `json:"development_mode"`
		}{
			State:           state,
			Initialized:     state == installation.StateReady,
			DevelopmentMode: developmentMode,
		})
	}
}

func requireInitialized(
	reader installation.StateReader,
	initializedOverride bool,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			state, err := effectiveInstallationState(request.Context(), reader, initializedOverride)
			if err != nil {
				writeError(response, http.StatusServiceUnavailable, "installation_state_unavailable", "The installation state could not be loaded.", "")
				return
			}
			if state != installation.StateReady {
				writeError(response, http.StatusServiceUnavailable, "core_not_initialized", "Kaeru Core setup must be completed before using this endpoint.", "")
				return
			}
			next.ServeHTTP(response, request)
		})
	}
}
