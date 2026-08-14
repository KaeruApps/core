package identity

import "context"

type Principal struct {
	Subject      string            `json:"subject"`
	Name         string            `json:"name"`
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
