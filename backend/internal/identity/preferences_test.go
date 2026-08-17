package identity

import (
	"context"
	"testing"
	"time"
)

type stubUserPreferencesStore struct {
	updated UserPreferences
}

func (store *stubUserPreferencesStore) GetUserPreferences(_ context.Context, _ string) (UserPreferences, error) {
	return UserPreferences{Username: "frog", TimeFormat: "24h", Timezone: "automatic", Theme: "dark"}, nil
}

func (store *stubUserPreferencesStore) UpdateUserPreferences(_ context.Context, _ string, preferences UserPreferences, _ time.Time) (UserPreferences, error) {
	store.updated = preferences
	return preferences, nil
}

func TestUserPreferencesManagerNormalizesOptionalProfileFields(t *testing.T) {
	store := &stubUserPreferencesStore{}
	manager := NewUserPreferencesManager(store)
	empty := "  "
	name := "  Frog Person  "

	preferences, err := manager.Update(context.Background(), "user-id", UserPreferences{
		Username:    "  frog  ",
		DisplayName: &name,
		Email:       &empty,
		TimeFormat:  "24h",
		Timezone:    "Europe/Amsterdam",
		Theme:       "dark",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if preferences.Username != "frog" || preferences.DisplayName == nil || *preferences.DisplayName != "Frog Person" {
		t.Fatalf("unexpected normalized profile: %#v", preferences)
	}
	if preferences.Email != nil {
		t.Fatalf("Email = %#v, want nil", preferences.Email)
	}
}

func TestUserPreferencesManagerRejectsInvalidTimezone(t *testing.T) {
	manager := NewUserPreferencesManager(&stubUserPreferencesStore{})
	_, err := manager.Update(context.Background(), "user-id", UserPreferences{
		Username: "frog", TimeFormat: "12h", Timezone: "somewhere nearby", Theme: "light",
	})
	validationError, ok := err.(*PreferencesValidationError)
	if !ok || validationError.Field != "timezone" {
		t.Fatalf("Update() error = %#v, want timezone validation error", err)
	}
}
