package registry

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	maxServiceTypeLength = 57
	maxInstanceIDLength  = 36
	maxNameLength        = 128
	maxVersionLength     = 64
	maxURLLength         = 2048
)

var (
	serviceTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	uuidPattern        = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

type ValidationError struct {
	Field   string
	Message string
}

func (validationError *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", validationError.Field, validationError.Message)
}

func ValidateRegistration(input RegistrationInput) error {
	if input.ServiceType == "" {
		return invalid("service_type", "is required")
	}
	if len(input.ServiceType) > maxServiceTypeLength || !serviceTypePattern.MatchString(input.ServiceType) {
		return invalid("service_type", "must start with a lowercase letter and contain only lowercase letters, numbers, underscores, or hyphens")
	}
	if input.ServiceType == CoreServiceType {
		return invalid("service_type", "is reserved for Kaeru Core")
	}
	if len(input.InstanceID) != maxInstanceIDLength || !uuidPattern.MatchString(input.InstanceID) {
		return invalid("instance_id", "must be a UUID")
	}
	if strings.TrimSpace(input.Name) == "" {
		return invalid("name", "is required")
	}
	if input.Name != strings.TrimSpace(input.Name) || len(input.Name) > maxNameLength {
		return invalid("name", "must be at most 128 characters without leading or trailing whitespace")
	}
	if strings.TrimSpace(input.Version) == "" || len(input.Version) > maxVersionLength {
		return invalid("version", "is required and must be at most 64 characters")
	}
	if err := validateHTTPURL(input.InternalURL, false); err != nil {
		return invalid("internal_url", err.Error())
	}
	return nil
}

func ValidateAccessURLs(publicURL string, nativeAppsURL string) error {
	if err := validateHTTPURL(publicURL, false); err != nil {
		return invalid("public_url", err.Error())
	}
	if err := validateHTTPURL(nativeAppsURL, true); err != nil {
		return invalid("native_apps_url", err.Error())
	}

	return nil
}

func validateHTTPURL(value string, optional bool) error {
	if value == "" && optional {
		return nil
	}
	if value == "" {
		return fmt.Errorf("is required")
	}
	if len(value) > maxURLLength {
		return fmt.Errorf("must be at most %d characters", maxURLLength)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("must not contain leading or trailing whitespace")
	}

	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("must be an absolute HTTP or HTTPS URL")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("must not include user information or a fragment")
	}

	return nil
}

func invalid(field string, message string) error {
	return &ValidationError{Field: field, Message: message}
}
