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

	maxAlternateURLGroups    = 50
	maxAlternateURLGroupName = 64
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
	if err := validateHTTPURL("Internal URL", input.InternalURL, false); err != nil {
		return invalid("internal_url", err.Error())
	}
	return nil
}

func ValidatePublicURL(publicURL string) error {
	if err := validateHTTPURL("Application URL", publicURL, false); err != nil {
		return invalid("public_url", err.Error())
	}

	return nil
}

// ValidateAlternateURLs checks a service's alternate URL submission.
//
// Kaeru Core owns the group list, so only Core may name groups or introduce
// new ones. Every other service submits a URL against a group that already
// exists and may leave it empty to fall back to its public URL.
func ValidateAlternateURLs(inputs []AlternateURLInput, isCore bool, knownGroups map[int64]string) error {
	if len(inputs) > maxAlternateURLGroups {
		return invalid("alternate_urls", fmt.Sprintf("There can be at most %d alternate URL groups.", maxAlternateURLGroups))
	}

	seenGroupIDs := make(map[int64]struct{}, len(inputs))
	seenNames := make(map[string]struct{}, len(inputs))
	for index, input := range inputs {
		field := fmt.Sprintf("alternate_urls[%d]", index)

		if !isCore {
			if input.GroupID == 0 {
				return invalid(field+".group_id", "Only Kaeru Core can create alternate URL groups.")
			}
			if _, exists := knownGroups[input.GroupID]; !exists {
				return invalid(field+".group_id", "Alternate URL groups must be ones Kaeru Core has defined.")
			}
		} else {
			name := strings.TrimSpace(input.Group)
			if name == "" {
				return invalid(field+".group", "Give every alternate URL a group.")
			}
			if name != input.Group || len(input.Group) > maxAlternateURLGroupName {
				return invalid(field+".group", fmt.Sprintf("Alternate URL group names must be at most %d characters without leading or trailing whitespace.", maxAlternateURLGroupName))
			}
			// Group names identify a group to administrators, so they are
			// compared without regard to case.
			lowered := strings.ToLower(name)
			if _, exists := seenNames[lowered]; exists {
				return invalid(field+".group", "Alternate URL group names must be unique.")
			}
			seenNames[lowered] = struct{}{}
			if input.GroupID != 0 {
				if _, exists := knownGroups[input.GroupID]; !exists {
					return invalid(field+".group_id", "Alternate URL groups must be ones Kaeru Core has defined.")
				}
			}
		}

		if input.GroupID != 0 {
			if _, exists := seenGroupIDs[input.GroupID]; exists {
				return invalid(field+".group_id", "Each alternate URL group can only appear once.")
			}
			seenGroupIDs[input.GroupID] = struct{}{}
		}

		if err := validateHTTPURL("Alternate URL", input.URL, true); err != nil {
			return invalid(field+".url", err.Error())
		}
	}

	return nil
}

// validateHTTPURL checks a URL, naming the field in every message so the text
// can be shown to an administrator as it is.
func validateHTTPURL(label string, value string, optional bool) error {
	if value == "" && optional {
		return nil
	}
	if value == "" {
		return fmt.Errorf("%s is required.", label)
	}
	if len(value) > maxURLLength {
		return fmt.Errorf("%s must be at most %d characters.", label, maxURLLength)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("%s must not contain leading or trailing whitespace.", label)
	}

	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s must be an absolute HTTP or HTTPS URL.", label)
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("%s must not include user information or a fragment.", label)
	}

	return nil
}

func invalid(field string, message string) error {
	return &ValidationError{Field: field, Message: message}
}
