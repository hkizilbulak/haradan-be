package router

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authn"
)

const APIBasePath = "/api"

// Options configures optional router dependencies.
type Options struct {
	AuthService *appauth.Service
}

// New builds the Gin engine with foundation middleware and OpenAPI routes.
func New(server generated.ServerInterface, logger *slog.Logger, opts ...Options) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(requestLogger(logger))

	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.AuthService != nil {
		protected := make([]authn.ProtectedRoute, 0,
			len(authn.AccountSessionProtectedRoutes)+len(authn.AdvertOwnerProtectedRoutes)+
				len(authn.MediaProtectedRoutes)+len(authn.FavoritesProtectedRoutes)+
				len(authn.AdvertModerationProtectedRoutes)+len(authn.PackagingAdminProtectedRoutes)+
				len(authn.AdvertUrgentProtectedRoutes)+len(authn.NotificationInboxProtectedRoutes)+
				len(authn.BannerAdminProtectedRoutes)+len(authn.AdminUserProtectedRoutes)+len(authn.TJKAdminProtectedRoutes))
		protected = append(protected, authn.AccountSessionProtectedRoutes...)
		protected = append(protected, authn.AdvertOwnerProtectedRoutes...)
		protected = append(protected, authn.MediaProtectedRoutes...)
		protected = append(protected, authn.FavoritesProtectedRoutes...)
		protected = append(protected, authn.AdvertModerationProtectedRoutes...)
		protected = append(protected, authn.PackagingAdminProtectedRoutes...)
		protected = append(protected, authn.BannerAdminProtectedRoutes...)
		protected = append(protected, authn.AdvertUrgentProtectedRoutes...)
		protected = append(protected, authn.NotificationInboxProtectedRoutes...)
		protected = append(protected, authn.AdminUserProtectedRoutes...)
		protected = append(protected, authn.TJKAdminProtectedRoutes...)
		r.Use(authn.Selective(opt.AuthService, logger, protected))
	}

	generated.RegisterHandlersWithOptions(r, server, generated.GinServerOptions{
		BaseURL:      APIBasePath,
		ErrorHandler: generatedParseErrorHandler(logger),
	})
	return r
}

// NewFoundation constructs a router with the foundation server and dependency checker.
// Geo and Catalog services are nil in foundation-only tests; those ops stay 501.
func NewFoundation(logger *slog.Logger, deps handler.DependencyChecker) *gin.Engine {
	return New(handler.NewServer(logger, deps, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil), logger)
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", time.Since(start).String(),
			"request_id", middleware.RequestIDFromContext(c.Request.Context()),
		)
	}
}

// CountOpenAPIRoutes returns the number of routes registered for OpenAPI operations.
func CountOpenAPIRoutes(engine *gin.Engine) int {
	count := 0
	for _, route := range engine.Routes() {
		if route.Path == "" {
			continue
		}
		count++
	}
	return count
}
