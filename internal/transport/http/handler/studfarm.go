package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// ListStudFarms delegates to the studfarm handler.
func (s *Server) ListStudFarms(c *gin.Context, params generated.ListStudFarmsParams) {
	if s.studfarm != nil {
		s.studfarm.ListStudFarms(c, params)
		return
	}
	s.NotImplementedServer.ListStudFarms(c, params)
}
