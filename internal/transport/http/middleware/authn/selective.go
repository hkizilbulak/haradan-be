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
	{Method: "POST", Path: "/api/v1/me/email/change-request"},
	{Method: "POST", Path: "/api/v1/me/password"},
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
	{Method: "PUT", Path: "/api/v1/me/adverts/:advertId/package"},
	{Method: "POST", Path: "/api/v1/me/adverts/:advertId/paytr/checkout"},
	{Method: "GET", Path: "/api/v1/me/adverts/:advertId/paytr/charges/:merchantOid"},
}

// MediaProtectedRoutes are MEDIA-01..07 owner-scoped routes. Admin media
// routes (/api/v1/admin/media/...) are intentionally excluded; they are out of
// scope here.
var MediaProtectedRoutes = []ProtectedRoute{
	{Method: "POST", Path: "/api/v1/media/uploads"},
	{Method: "POST", Path: "/api/v1/media/assets/:assetId/confirm"},
	{Method: "PUT", Path: "/api/v1/media/assets/:assetId/content"},
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

// NotificationInboxProtectedRoutes are authenticated ACTIVE-user inbox routes.
var NotificationInboxProtectedRoutes = []ProtectedRoute{
	{Method: "GET", Path: "/api/v1/me/notifications"},
	{Method: "GET", Path: "/api/v1/me/notifications/unread-count"},
	{Method: "PUT", Path: "/api/v1/me/notifications/read-all"},
	{Method: "PUT", Path: "/api/v1/me/notifications/:notificationId/read"},
}

// AdvertModerationProtectedRoutes are ADVERT-ADMIN-01..06 BO_AUTH routes.
// Admin media/banner/user management routes are intentionally excluded.
var AdvertModerationProtectedRoutes = []ProtectedRoute{
	{Method: "GET", Path: "/api/v1/admin/adverts/moderation"},
	{Method: "GET", Path: "/api/v1/admin/adverts/:advertId"},
	{Method: "POST", Path: "/api/v1/admin/adverts/:advertId/approve"},
	{Method: "POST", Path: "/api/v1/admin/adverts/:advertId/request-changes"},
	{Method: "POST", Path: "/api/v1/admin/adverts/:advertId/reject"},
	{Method: "POST", Path: "/api/v1/admin/adverts/:advertId/suspend"},
}

// AdminUserProtectedRoutes are ADMIN-USER BO_AUTH routes.
var AdminUserProtectedRoutes = []ProtectedRoute{
	{Method: "GET", Path: "/api/v1/admin/users"},
	{Method: "POST", Path: "/api/v1/admin/users"},
	{Method: "GET", Path: "/api/v1/admin/users/:userId"},
	{Method: "PATCH", Path: "/api/v1/admin/users/:userId"},
	{Method: "POST", Path: "/api/v1/admin/users/:userId/role"},
	{Method: "POST", Path: "/api/v1/admin/users/:userId/status"},
	{Method: "GET", Path: "/api/v1/admin/users/:userId/security-events"},
	{Method: "POST", Path: "/api/v1/admin/users/:userId/invitation/resend"},
	{Method: "POST", Path: "/api/v1/admin/users/:userId/email/change-request"},
}

var TJKAdminProtectedRoutes = []ProtectedRoute{
	{Method: "POST", Path: "/api/v1/admin/tjk/sync-runs"},
	{Method: "GET", Path: "/api/v1/admin/tjk/sync-runs"},
	{Method: "GET", Path: "/api/v1/admin/tjk/sync-runs/:runId"},
	{Method: "POST", Path: "/api/v1/admin/tjk/sync-runs/:runId/cancel"},
	{Method: "GET", Path: "/api/v1/admin/tjk/sync-runs/:runId/item-errors"},
	{Method: "POST", Path: "/api/v1/admin/tjk/item-errors/:errorId/resolve"},
	{Method: "POST", Path: "/api/v1/admin/tjk/item-errors/:errorId/ignore"},
}

// PackagingAdminProtectedRoutes are packaging/campaign/notification-template/job BO_AUTH routes.
var PackagingAdminProtectedRoutes = []ProtectedRoute{
	{Method: "GET", Path: "/api/v1/admin/packages"},
	{Method: "POST", Path: "/api/v1/admin/packages"},
	{Method: "PUT", Path: "/api/v1/admin/packages/reorder"},
	{Method: "GET", Path: "/api/v1/admin/packages/:packageCode"},
	{Method: "PATCH", Path: "/api/v1/admin/packages/:packageCode"},
	{Method: "GET", Path: "/api/v1/admin/adverts/:advertId/package"},
	{Method: "PUT", Path: "/api/v1/admin/adverts/:advertId/package"},
	{Method: "GET", Path: "/api/v1/admin/adverts/:advertId/package-history"},
	{Method: "POST", Path: "/api/v1/admin/adverts/:advertId/package/cancel"},
	{Method: "GET", Path: "/api/v1/admin/campaigns"},
	{Method: "POST", Path: "/api/v1/admin/campaigns"},
	{Method: "GET", Path: "/api/v1/admin/campaigns/:campaignId"},
	{Method: "PATCH", Path: "/api/v1/admin/campaigns/:campaignId"},
	{Method: "GET", Path: "/api/v1/admin/notification-templates"},
	{Method: "GET", Path: "/api/v1/admin/notification-templates/:eventType"},
	{Method: "PATCH", Path: "/api/v1/admin/notification-templates/:eventType"},
	{Method: "GET", Path: "/api/v1/admin/email-templates/provider"},
	{Method: "GET", Path: "/api/v1/admin/email-templates/provider/:templateId/variables"},
	{Method: "GET", Path: "/api/v1/admin/jobs"},
	{Method: "GET", Path: "/api/v1/admin/jobs/:jobId"},
	{Method: "PATCH", Path: "/api/v1/admin/jobs/:jobId"},
	{Method: "POST", Path: "/api/v1/admin/jobs/:jobId/run"},
	{Method: "GET", Path: "/api/v1/admin/jobs/:jobId/history"},
}

// BannerAdminProtectedRoutes are banner management BO_AUTH routes.
var BannerAdminProtectedRoutes = []ProtectedRoute{
	{Method: "GET", Path: "/api/v1/admin/banners"},
	{Method: "POST", Path: "/api/v1/admin/banners"},
	{Method: "PUT", Path: "/api/v1/admin/banners/reorder"},
	{Method: "GET", Path: "/api/v1/admin/banners/:bannerId"},
	{Method: "PATCH", Path: "/api/v1/admin/banners/:bannerId"},
	{Method: "POST", Path: "/api/v1/admin/banners/:bannerId/status"},
}

// CatalogAdminProtectedRoutes are ADMIN-CATALOG-* BO_AUTH category/property routes.
// Without these, Bearer tokens from the BO proxy never become a Principal and
// requireAdminBO returns 401 "Kimlik doğrulama gerekli."
var CatalogAdminProtectedRoutes = []ProtectedRoute{
	{Method: "GET", Path: "/api/v1/admin/categories"},
	{Method: "POST", Path: "/api/v1/admin/categories"},
	{Method: "PUT", Path: "/api/v1/admin/categories/reorder"},
	{Method: "GET", Path: "/api/v1/admin/categories/:categoryId"},
	{Method: "PATCH", Path: "/api/v1/admin/categories/:categoryId"},
	{Method: "POST", Path: "/api/v1/admin/categories/:categoryId/active"},
	{Method: "POST", Path: "/api/v1/admin/categories/:categoryId/reparent"},
	{Method: "GET", Path: "/api/v1/admin/categories/:categoryId/properties"},
	{Method: "POST", Path: "/api/v1/admin/categories/:categoryId/properties"},
	{Method: "PUT", Path: "/api/v1/admin/categories/:categoryId/properties/reorder"},
	{Method: "PATCH", Path: "/api/v1/admin/categories/:categoryId/properties/:propertyId"},
	{Method: "POST", Path: "/api/v1/admin/categories/:categoryId/properties/:propertyId/active"},
}

// MediaAdminProtectedRoutes are admin media upload/status BO_AUTH routes.
var MediaAdminProtectedRoutes = []ProtectedRoute{
	{Method: "POST", Path: "/api/v1/admin/media/uploads"},
	{Method: "POST", Path: "/api/v1/admin/media/assets/:assetId/confirm"},
	{Method: "GET", Path: "/api/v1/admin/media/assets/:assetId"},
}

// AdvertUrgentProtectedRoutes are owner/admin URGENT activate/deactivate routes.
var AdvertUrgentProtectedRoutes = []ProtectedRoute{
	{Method: "PUT", Path: "/api/v1/adverts/:advertId/urgent"},
	{Method: "DELETE", Path: "/api/v1/adverts/:advertId/urgent"},
}

var CouponAdminProtectedRoutes = []ProtectedRoute{
	{Method: "GET", Path: "/api/v1/admin/coupons"},
	{Method: "POST", Path: "/api/v1/admin/coupons"},
	{Method: "GET", Path: "/api/v1/admin/coupons/:id"},
	{Method: "PUT", Path: "/api/v1/admin/coupons/:id"},
	{Method: "PATCH", Path: "/api/v1/admin/coupons/:id/active"},
}

var CouponUserProtectedRoutes = []ProtectedRoute{
	{Method: "POST", Path: "/api/v1/coupons/validate"},
}

var AdvertCommentProtectedRoutes = []ProtectedRoute{
	{Method: "POST", Path: "/api/v1/adverts/:advertId/comments"},
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

// PublicFavoriteEnrichmentRoutes are PUBLIC routes that optionally accept Bearer
// so PublishedAdvertCard.isFavorite can be enriched for logged-in buyers.
var PublicFavoriteEnrichmentRoutes = []ProtectedRoute{
	{Method: "GET", Path: "/api/v1/adverts"},
	{Method: "GET", Path: "/api/v1/adverts/:advertId"},
	{Method: "GET", Path: "/api/v1/homepage/new-adverts"},
	{Method: "GET", Path: "/api/v1/homepage/showcase"},
	{Method: "GET", Path: "/api/v1/homepage/urgent"},
	{Method: "GET", Path: "/api/v1/homepage/featured"},
}

// OptionalSelective runs soft Bearer auth on the listed public routes.
func OptionalSelective(svc *appauth.Service, logger *slog.Logger, routes []ProtectedRoute) gin.HandlerFunc {
	allow := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		allow[r.Method+" "+r.Path] = struct{}{}
	}
	inner := OptionalMiddleware(svc, logger)
	return func(c *gin.Context) {
		key := c.Request.Method + " " + c.FullPath()
		if _, ok := allow[key]; !ok {
			c.Next()
			return
		}
		inner(c)
	}
}
