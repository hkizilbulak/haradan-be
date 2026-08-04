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

// AdvertOwnerProtectedRoutes are ADVERT-OWNER-01..11 routes. Media relation and
// public/search routes are intentionally excluded; they are out of scope here.
var AdvertOwnerProtectedRoutes = []ProtectedRoute{
	{Method: "POST", Path: "/api/v1/me/adverts"},
	{Method: "GET", Path: "/api/v1/me/adverts"},
	{Method: "GET", Path: "/api/v1/me/adverts/:advertId"},
	{Method: "PATCH", Path: "/api/v1/me/adverts/:advertId"},
	{Method: "PUT", Path: "/api/v1/me/adverts/:advertId/category"},
	{Method: "PUT", Path: "/api/v1/me/adverts/:advertId/properties"},
	{Method: "POST", Path: "/api/v1/me/adverts/:advertId/submit"},
	{Method: "POST", Path: "/api/v1/me/adverts/:advertId/resubmit"},
	{Method: "DELETE", Path: "/api/v1/me/adverts/:advertId"},
	{Method: "POST", Path: "/api/v1/me/adverts/:advertId/sold"},
	{Method: "POST", Path: "/api/v1/me/adverts/:advertId/archive"},
}

// MediaProtectedRoutes are MEDIA-01..07 owner-scoped routes. Admin media
// routes (/api/v1/admin/media/...) are intentionally excluded; they are out of
// scope here.
var MediaProtectedRoutes = []ProtectedRoute{
	{Method: "POST", Path: "/api/v1/media/uploads"},
	{Method: "POST", Path: "/api/v1/media/assets/:assetId/confirm"},
	{Method: "GET", Path: "/api/v1/media/assets/:assetId"},
	{Method: "POST", Path: "/api/v1/me/adverts/:advertId/media"},
	{Method: "DELETE", Path: "/api/v1/me/adverts/:advertId/media/:assetId"},
	{Method: "PUT", Path: "/api/v1/me/adverts/:advertId/media/order"},
	{Method: "PUT", Path: "/api/v1/me/adverts/:advertId/media/cover"},
}

// FavoritesProtectedRoutes are FAVORITE-01..03 authenticated user routes.
var FavoritesProtectedRoutes = []ProtectedRoute{
	{Method: "GET", Path: "/api/v1/me/favorites"},
	{Method: "PUT", Path: "/api/v1/me/favorites/:advertId"},
	{Method: "DELETE", Path: "/api/v1/me/favorites/:advertId"},
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
