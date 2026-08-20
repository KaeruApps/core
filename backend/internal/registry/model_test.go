package registry

import (
	"errors"
	"strings"
	"testing"
)

func TestResolveAlternateURL(t *testing.T) {
	details := ServiceDetails{
		Service: Service{PublicURL: "https://upload.example.com"},
		AlternateURLs: []ServiceAlternateURL{
			{GroupID: 1, Group: "Native", URL: "https://upload-native.example.com"},
			{GroupID: 2, Group: "LAN"},
		},
	}

	if got := details.ResolveAlternateURL(1); got != "https://upload-native.example.com" {
		t.Errorf("ResolveAlternateURL(1) = %q, want the group's URL", got)
	}
	// A group the service left blank falls back to its public URL.
	if got := details.ResolveAlternateURL(2); got != details.PublicURL {
		t.Errorf("ResolveAlternateURL(2) = %q, want the public URL", got)
	}
	// So does a group the service knows nothing about.
	if got := details.ResolveAlternateURL(99); got != details.PublicURL {
		t.Errorf("ResolveAlternateURL(99) = %q, want the public URL", got)
	}
}

func TestValidatePublicURL(t *testing.T) {
	if err := ValidatePublicURL("https://upload.example.com"); err != nil {
		t.Errorf("ValidatePublicURL(valid) error = %v", err)
	}
	for name, value := range map[string]string{"empty": "", "not a URL": "not-a-url", "no scheme": "upload.example.com"} {
		if err := ValidatePublicURL(value); err == nil {
			t.Errorf("ValidatePublicURL(%s) accepted %q", name, value)
		}
	}
}

func TestValidateAlternateURLsForCore(t *testing.T) {
	known := map[int64]string{1: "Native"}

	valid := []AlternateURLInput{
		{GroupID: 1, Group: "Native", URL: "https://core-native.example.com"},
		{Group: "LAN", URL: "https://core.lan"},
		{Group: "Remote"}, // no URL falls back to the public URL
	}
	if err := ValidateAlternateURLs(valid, true, known); err != nil {
		t.Errorf("ValidateAlternateURLs(valid) error = %v", err)
	}

	cases := map[string][]AlternateURLInput{
		"missing group name":    {{Group: ""}},
		"untrimmed group name":  {{Group: " Native "}},
		"duplicate name":        {{Group: "Native"}, {Group: "Native"}},
		"duplicate name casing": {{Group: "Native"}, {Group: "native"}},
		"duplicate group id":    {{GroupID: 1, Group: "Native"}, {GroupID: 1, Group: "Other"}},
		"unknown group id":      {{GroupID: 42, Group: "Native"}},
		"invalid url":           {{Group: "Native", URL: "not-a-url"}},
	}
	for name, inputs := range cases {
		if err := ValidateAlternateURLs(inputs, true, known); err == nil {
			t.Errorf("ValidateAlternateURLs(%s) accepted an invalid submission", name)
		}
	}
}

func TestValidateAlternateURLsForOtherServices(t *testing.T) {
	known := map[int64]string{1: "Native"}

	if err := ValidateAlternateURLs([]AlternateURLInput{{GroupID: 1, URL: "https://upload-native.example.com"}}, false, known); err != nil {
		t.Errorf("ValidateAlternateURLs(valid) error = %v", err)
	}
	// Only Kaeru Core may introduce a group.
	if err := ValidateAlternateURLs([]AlternateURLInput{{Group: "Invented", URL: "https://x.example.com"}}, false, known); err == nil {
		t.Error("a service other than Core was allowed to create an alternate URL group")
	}
	if err := ValidateAlternateURLs([]AlternateURLInput{{GroupID: 42, URL: "https://x.example.com"}}, false, known); err == nil {
		t.Error("a service was allowed to reference an unknown alternate URL group")
	}
}

// Validation messages are rendered to administrators verbatim, so each one has
// to name what is wrong on its own rather than read as a sentence fragment
// that only makes sense next to its field name.
func TestServiceUpdateValidationMessagesAreSelfContained(t *testing.T) {
	roles := []ServiceRole{{Key: "viewer", Name: "Viewer", Active: true}}
	unknownRole := "nope"

	cases := map[string]UpdateServiceInput{
		"missing application URL": {},
		"invalid application URL": {PublicURL: "not-a-url"},
		"unknown default role":    {PublicURL: "https://upload.example.com", DefaultRoleKey: &unknownRole},
		"unknown mapped role": {
			PublicURL:    "https://upload.example.com",
			RoleMappings: []ServiceRoleMapping{{RoleKey: "nope", OIDCGroups: []string{"admins"}}},
		},
		"role mapping without groups": {
			PublicURL:    "https://upload.example.com",
			RoleMappings: []ServiceRoleMapping{{RoleKey: "viewer"}},
		},
		"duplicate mapped role": {
			PublicURL: "https://upload.example.com",
			RoleMappings: []ServiceRoleMapping{
				{RoleKey: "viewer", OIDCGroups: []string{"a"}},
				{RoleKey: "viewer", OIDCGroups: []string{"b"}},
			},
		},
		"blank OIDC group": {
			PublicURL:    "https://upload.example.com",
			RoleMappings: []ServiceRoleMapping{{RoleKey: "viewer", OIDCGroups: []string{"  "}}},
		},
		"invalid alternate URL": {
			PublicURL:     "https://upload.example.com",
			AlternateURLs: []AlternateURLInput{{Group: "Native", URL: "not-a-url"}},
		},
		"blank alternate URL group": {
			PublicURL:     "https://upload.example.com",
			AlternateURLs: []AlternateURLInput{{Group: ""}},
		},
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateServiceUpdate(input, roles, true, nil)
			var validationError *ValidationError
			if !errors.As(err, &validationError) {
				t.Fatalf("ValidateServiceUpdate() error = %v, want a validation error", err)
			}
			message := validationError.Message
			if message == "" {
				t.Fatal("validation error has no message")
			}
			// A fragment starts with a verb such as "is" or "must"; a message
			// that can stand on its own names its subject first.
			for _, fragment := range []string{"is ", "must ", "cannot ", "should "} {
				if strings.HasPrefix(message, fragment) {
					t.Errorf("message %q reads as a fragment; it should name the field it is about", message)
				}
			}
			if !strings.HasSuffix(message, ".") {
				t.Errorf("message %q should end with a full stop", message)
			}
		})
	}
}
