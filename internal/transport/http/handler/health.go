package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware"
)

// GetHealth returns liveness when dependencies are reachable; otherwise 503.
func (s *Server) GetHealth(c *gin.Context) {
	traceID := middleware.RequestIDFromContext(c.Request.Context())
	if s.deps == nil {
		c.JSON(http.StatusServiceUnavailable, generated.ErrorResponse{
			Code:    generated.DomainErrorCodeDEPENDENCYUNAVAILABLE,
			Message: "Bağımlılık şu anda kullanılamıyor.",
			TraceId: traceID,
		})
		return
	}
	if err := s.deps.Ping(c.Request.Context()); err != nil {
		if s.logger != nil {
			s.logger.Error("health dependency check failed", "request_id", traceID)
		}
		c.JSON(http.StatusServiceUnavailable, generated.ErrorResponse{
			Code:    generated.DomainErrorCodeDEPENDENCYUNAVAILABLE,
			Message: "Bağımlılık şu anda kullanılamıyor.",
			TraceId: traceID,
		})
		return
	}

	c.JSON(http.StatusOK, generated.HealthResponse{
		Status: generated.Ok,
	})
}
