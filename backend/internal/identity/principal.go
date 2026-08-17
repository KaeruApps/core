package identity

import "context"

type Principal struct {
	ID           string            `json:"id,omitempty"`
	Subject      string            `json:"subject"`
	Name         string            `json:"name"`
	DisplayName  *string           `json:"display_name,omitempty"`
	Email        *string           `json:"email,omitempty"`
	AvatarURL    *string           `json:"avatar_url,omitempty"`
	ServiceRoles map[string]string `json:"service_roles"`
}

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func FromContext(ctx context.Context) (Principal, bool) {
	principal, exists := ctx.Value(principalContextKey{}).(Principal)
	return principal, exists
}

func DevelopmentPrincipal() Principal {
	return Principal{
		Subject: "kaeru-development-user",
		Name:    "Local Developer",
		ServiceRoles: map[string]string{
			"core": "admin",
		},
	}
}
