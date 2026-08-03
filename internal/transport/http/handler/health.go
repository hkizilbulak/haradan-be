package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// GetHealth returns a liveness-oriented healthy response without dependency probes.
func (s *Server) GetHealth(c *gin.Context) {
	c.JSON(http.StatusOK, generated.HealthResponse{
		Status: generated.Ok,
	})
}
