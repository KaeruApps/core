package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/KaeruApps/core/internal/identity"
)

const maxUserAvatarRequestBytes = 6 * 1024 * 1024

func getUserAvatar(manager UserAvatarManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		principal, authenticated := identity.FromContext(request.Context())
		if !authenticated || principal.ID == "" {
			writeError(response, http.StatusUnauthorized, "authentication_required", "Authentication is required.", "")
			return
		}
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "avatar_unavailable", "The user avatar is unavailable.", "")
			return
		}
		avatar, err := manager.Get(request.Context(), principal.ID)
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

func updateUserAvatar(manager UserAvatarManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		principal, authenticated := identity.FromContext(request.Context())
		if !authenticated || principal.ID == "" {
			writeError(response, http.StatusUnauthorized, "authentication_required", "Authentication is required.", "")
			return
		}
		if manager == nil {
			writeError(response, http.StatusServiceUnavailable, "avatar_unavailable", "The user avatar is unavailable.", "")
			return
		}

		request.Body = http.MaxBytesReader(response, request.Body, maxUserAvatarRequestBytes)
		if err := request.ParseMultipartForm(maxUserAvatarRequestBytes); err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				writeError(response, http.StatusRequestEntityTooLarge, "request_too_large", "Avatar upload must be 5 MB or smaller.", "avatar")
				return
			}
			writeError(response, http.StatusBadRequest, "invalid_request", "The avatar upload must contain valid multipart form data.", "avatar")
			return
		}
		if request.MultipartForm != nil {
			defer request.MultipartForm.RemoveAll()
		}
		file, _, err := request.FormFile("avatar")
		if err != nil {
			writeError(response, http.StatusBadRequest, "invalid_request", "Choose an avatar image to upload.", "avatar")
			return
		}
		defer file.Close()
		content, err := io.ReadAll(io.LimitReader(file, identity.MaxUserAvatarUploadBytes+1))
		if err != nil {
			writeError(response, http.StatusBadRequest, "invalid_request", "The avatar image could not be read.", "avatar")
			return
		}
		avatar, err := manager.Update(request.Context(), principal.ID, content)
		if err == nil {
			writeJSON(response, http.StatusOK, struct {
				AvatarURL string `json:"avatar_url"`
			}{AvatarURL: fmt.Sprintf("/api/v1/users/me/avatar?v=%d", avatar.UpdatedAt.UnixNano())})
			return
		}
		if errors.Is(err, identity.ErrUserAvatarInvalid) {
			writeError(response, http.StatusBadRequest, "validation_failed", "Avatar must be a valid JPG or PNG file no larger than 5 MB.", "avatar")
			return
		}
		writeError(response, http.StatusInternalServerError, "internal_error", "The user avatar could not be saved.", "")
	}
}
