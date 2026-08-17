package identity

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	_ "time/tzdata"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrUsernameTaken = errors.New("username is already in use")
)

type UserPreferences struct {
	Username    string  `json:"username"`
	DisplayName *string `json:"name"`
	Email       *string `json:"email"`
	TimeFormat  string  `json:"time_format"`
	Timezone    string  `json:"timezone"`
	Theme       string  `json:"theme"`
}

type PreferencesValidationError struct {
	Field   string
	Message string
}

func (err *PreferencesValidationError) Error() string { return err.Message }

type UserPreferencesStore interface {
	GetUserPreferences(context.Context, string) (UserPreferences, error)
	UpdateUserPreferences(context.Context, string, UserPreferences, time.Time) (UserPreferences, error)
}

type UserPreferencesManager struct {
	store UserPreferencesStore
	now   func() time.Time
}

func NewUserPreferencesManager(store UserPreferencesStore) *UserPreferencesManager {
	return &UserPreferencesManager{
		store: store,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func (manager *UserPreferencesManager) Get(ctx context.Context, userID string) (UserPreferences, error) {
	return manager.store.GetUserPreferences(ctx, userID)
}

func (manager *UserPreferencesManager) Update(ctx context.Context, userID string, input UserPreferences) (UserPreferences, error) {
	input.Username = strings.TrimSpace(input.Username)
	if input.Username == "" || len(input.Username) > 255 {
		return UserPreferences{}, invalidPreference("username", "Username must contain between 1 and 255 characters.")
	}

	input.DisplayName = optionalPreference(input.DisplayName)
	if input.DisplayName != nil && len(*input.DisplayName) > 255 {
		return UserPreferences{}, invalidPreference("name", "Name must contain no more than 255 characters.")
	}

	input.Email = optionalPreference(input.Email)
	if input.Email != nil {
		if len(*input.Email) > 320 {
			return UserPreferences{}, invalidPreference("email", "Email must contain no more than 320 characters.")
		}
		address, err := mail.ParseAddress(*input.Email)
		if err != nil || address.Address != *input.Email {
			return UserPreferences{}, invalidPreference("email", "Email must be a valid email address.")
		}
	}

	if input.TimeFormat != "12h" && input.TimeFormat != "24h" {
		return UserPreferences{}, invalidPreference("time_format", "Time format must be 12-hour or 24-hour.")
	}
	input.Timezone = strings.TrimSpace(input.Timezone)
	if input.Timezone == "" {
		return UserPreferences{}, invalidPreference("timezone", "Timezone is required.")
	}
	if input.Timezone != "automatic" {
		if len(input.Timezone) > 255 {
			return UserPreferences{}, invalidPreference("timezone", "Timezone is not valid.")
		}
		if _, err := time.LoadLocation(input.Timezone); err != nil {
			return UserPreferences{}, invalidPreference("timezone", "Timezone must be automatic or a valid IANA timezone.")
		}
	}
	if input.Theme != "dark" && input.Theme != "light" {
		return UserPreferences{}, invalidPreference("theme", "Theme must be dark or light.")
	}

	return manager.store.UpdateUserPreferences(ctx, userID, input, manager.now())
}

func optionalPreference(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func invalidPreference(field, message string) error {
	return &PreferencesValidationError{Field: field, Message: message}
}
