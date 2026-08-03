package router

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware"
)

const APIBasePath = "/api"

// New builds the Gin engine with foundation middleware and OpenAPI routes.
func New(server generated.ServerInterface, logger *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(requestLogger(logger))

	generated.RegisterHandlersWithOptions(r, server, generated.GinServerOptions{
		BaseURL: APIBasePath,
	})
	return r
}

// NewFoundation constructs a router with the foundation server and dependency checker.
func NewFoundation(logger *slog.Logger, deps handler.DependencyChecker) *gin.Engine {
	return New(handler.NewServer(logger, deps), logger)
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
