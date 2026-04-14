package http

import (
	"context"

	"github.com/vvs/isp/internal/modules/auth/domain"
)

type contextKey struct{}

// UserFromContext retrieves the authenticated user stored by RequireAuth middleware.
func UserFromContext(ctx context.Context) *domain.User {
	u, _ := ctx.Value(contextKey{}).(*domain.User)
	return u
}

// WithUser returns a context with the user stored.
func WithUser(ctx context.Context, u *domain.User) context.Context {
	return context.WithValue(ctx, contextKey{}, u)
}

// userFromContext is a handler-internal convenience wrapper.
func userFromContext(r interface{ Context() context.Context }) *domain.User {
	return UserFromContext(r.Context())
}
