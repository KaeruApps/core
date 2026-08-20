package api

import (
	"net/http"
)

func listBackupOptions(directory BackupOptionsDirectory) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if directory == nil {
			writeError(response, http.StatusServiceUnavailable, "backup_options_unavailable", "Backup options are unavailable.", "")
			return
		}
		options, err := directory.List(request.Context())
		if err != nil {
			writeError(response, http.StatusInternalServerError, "internal_error", "Backup options could not be loaded.", "")
			return
		}
		writeJSON(response, http.StatusOK, options)
	}
}
