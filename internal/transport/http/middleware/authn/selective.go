package authn

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
)

// ProtectedRoute is an exact method + gin FullPath pair (including /api prefix).
type ProtectedRoute struct {
	Method string
	Path   string
}

// AccountSessionProtectedRoutes are FE_AUTH routes implemented in the account/session slice.
var AccountSessionProtectedRoutes = []ProtectedRoute{
	{Method: "GET", Path: "/api/v1/me"},
	{Method: "PATCH", Path: "/api/v1/me"},
	{Method: "POST", Path: "/api/v1/auth/logout-all"},
	{Method: "GET", Path: "/api/v1/me/sessions"},
	{Method: "DELETE", Path: "/api/v1/me/sessions/:sessionId"},
}

// Selective runs Bearer access-token auth only for the listed method+path pairs.
// Unlisted routes (including public Health/Geo/Catalog/Auth and remaining 501 FE_AUTH
// stubs) are left untouched.
func Selective(svc *appauth.Service, logger *slog.Logger, routes []ProtectedRoute) gin.HandlerFunc {
	allow := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		allow[r.Method+" "+r.Path] = struct{}{}
	}
	inner := Middleware(svc, logger)
	return func(c *gin.Context) {
		key := c.Request.Method + " " + c.FullPath()
		if _, ok := allow[key]; !ok {
			c.Next()
			return
		}
		inner(c)
	}
}
