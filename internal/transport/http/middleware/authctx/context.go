package authctx

import (
	"context"

	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
)

type ctxKey struct{}

// WithPrincipal stores the authenticated principal in context.
func WithPrincipal(ctx context.Context, p domainauth.Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// PrincipalFromContext returns the authenticated principal if present.
func PrincipalFromContext(ctx context.Context) (domainauth.Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(domainauth.Principal)
	return p, ok
}
