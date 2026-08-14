package config

import "testing"

func TestLoadRuntimeDevelopmentAuth(t *testing.T) {
	t.Setenv(developmentAuthEnvironmentVariable, "true")

	runtime, err := LoadRuntime()
	if err != nil {
		t.Fatalf("LoadRuntime() error = %v", err)
	}
	if !runtime.DevelopmentAuth {
		t.Fatal("expected development authentication to be enabled")
	}
}

func TestLoadRuntimeRejectsInvalidDevelopmentAuth(t *testing.T) {
	t.Setenv(developmentAuthEnvironmentVariable, "sometimes")

	if _, err := LoadRuntime(); err == nil {
		t.Fatal("LoadRuntime() expected an error")
	}
}
