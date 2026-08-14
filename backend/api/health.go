package api

import "net/http"

func health(initialized bool, developmentMode bool) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
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
