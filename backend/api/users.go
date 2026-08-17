package api

import (
	"errors"
	"net/http"

	"github.com/KaeruApps/core/internal/identity"
	"github.com/go-chi/chi/v5"
)

func listUsers(directory UserDirectory) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if directory == nil {
			writeError(response, http.StatusServiceUnavailable, "users_unavailable", "The user directory is unavailable.", "")
			return
		}
		users, err := directory.List(request.Context())
		if err != nil {
			writeError(response, http.StatusInternalServerError, "internal_error", "Users could not be loaded.", "")
			return
		}
		writeJSON(response, http.StatusOK, users)
	}
}

func getManagedUserAvatar(manager UserAvatarManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "avatar_unavailable", "The user avatar is unavailable.", "")
			return
		}
		avatar, err := manager.Get(request.Context(), chi.URLParam(request, "userID"))
		if err == nil {
			response.Header().Set("Content-Type", avatar.ContentType)
			response.Header().Set("Cache-Control", "private, max-age=300")
			response.WriteHeader(http.StatusOK)
			_, _ = response.Write(avatar.Content)
			return
		}
		if errors.Is(err, identity.ErrUserAvatarNotFound) {
			writeError(response, http.StatusNotFound, "avatar_not_found", "The user has no uploaded avatar.", "")
			return
		}
		writeError(response, http.StatusInternalServerError, "internal_error", "The user avatar could not be loaded.", "")
	}
}
