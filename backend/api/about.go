package api

import "net/http"

type ApplicationInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

func about(application ApplicationInfo) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		writeJSON(response, http.StatusOK, application)
	}
}
