package api

import (
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"

	"github.com/KaeruApps/core/internal/registry"
	"github.com/go-chi/chi/v5"
)

func getServiceIcon(manager ServiceIconManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "service_icons_unavailable", "Service icons are unavailable.", "")
			return
		}

		icon, err := manager.Get(request.Context(), chi.URLParam(request, "serviceID"))
		if err != nil {
			switch {
			case errors.Is(err, registry.ErrServiceNotFound):
				writeError(response, http.StatusNotFound, "service_not_found", "The requested service was not found.", "service_id")
			case errors.Is(err, registry.ErrServiceIconNotFound):
				writeError(response, http.StatusNotFound, "service_icon_not_found", "The service does not have a cached icon.", "service_id")
			default:
				writeError(response, http.StatusInternalServerError, "internal_error", "The service icon could not be loaded.", "")
			}
			return
		}

		etag := `"` + hex.EncodeToString(icon.ContentHash[:]) + `"`
		if request.Header.Get("If-None-Match") == etag {
			response.WriteHeader(http.StatusNotModified)
			return
		}
		response.Header().Set("Content-Type", icon.ContentType)
		response.Header().Set("Content-Length", strconv.Itoa(len(icon.Content)))
		response.Header().Set("Cache-Control", "private, max-age=300")
		response.Header().Set("ETag", etag)
		response.Header().Set("X-Content-Type-Options", "nosniff")
		if icon.ContentType == "image/svg+xml" {
			response.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src data:; style-src 'unsafe-inline'")
		}
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(icon.Content)
	}
}
