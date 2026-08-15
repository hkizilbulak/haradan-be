package authn

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

// Middleware validates Bearer access tokens for protected routes.
// Do not register globally on all OpenAPI routes; wire selectively when real FE_AUTH
// endpoints beyond this slice are implemented. LogoutCurrentSession authenticates
// inside its handler via the shared application AuthenticateAccessToken path.
func Middleware(svc *appauth.Service, _ *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, err := bind.BearerToken(c)
		if err != nil {
			writeAuthError(c, err)
			c.Abort()
			return
		}
		principal, err := svc.AuthenticateAccessToken(c.Request.Context(), tok)
		if err != nil {
			writeAuthError(c, err)
			c.Abort()
			return
		}
		ctx := authctx.WithPrincipal(c.Request.Context(), principal)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// OptionalMiddleware enriches the request with a Principal when a valid Bearer
// token is present. Missing Authorization is allowed (anonymous). A present but
// invalid/revoked token is rejected so callers cannot silently appear anonymous.
func OptionalMiddleware(svc *appauth.Service, _ *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if strings.TrimSpace(header) == "" {
			c.Next()
			return
		}
		tok, err := bind.BearerToken(c)
		if err != nil {
			writeAuthError(c, err)
			c.Abort()
			return
		}
		principal, err := svc.AuthenticateAccessToken(c.Request.Context(), tok)
		if err != nil {
			writeAuthError(c, err)
			c.Abort()
			return
		}
		ctx := authctx.WithPrincipal(c.Request.Context(), principal)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func writeAuthError(c *gin.Context, err error) {
	traceID := middleware.RequestIDFromContext(c.Request.Context())
	ae, ok := apperr.As(err)
	if !ok {
		c.JSON(http.StatusUnauthorized, generated.ErrorResponse{
			Code:    generated.DomainErrorCodeUNAUTHENTICATED,
			Message: "Kimlik doğrulama gerekli.",
			TraceId: traceID,
		})
		return
	}
	status := http.StatusUnauthorized
	if ae.Kind == apperr.KindForbidden {
		status = http.StatusForbidden
	}
	c.JSON(status, generated.ErrorResponse{
		Code:    generated.DomainErrorCode(ae.Code),
		Message: ae.Message,
		TraceId: traceID,
	})
}

// ExtractBearer is exported for unit tests.
func ExtractBearer(header string) (string, bool) {
	h := strings.TrimSpace(header)
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	tok := strings.TrimSpace(h[len(prefix):])
	return tok, tok != ""
}
