package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// SearchHorsesForSelection implements HORSE-01.
func (s *Server) SearchHorsesForSelection(c *gin.Context, params generated.SearchHorsesForSelectionParams) {
	if s.horse == nil {
		respondNotImplemented(c)
		return
	}
	s.horse.SearchHorsesForSelection(c, params)
}

// GetHorsePublicDetail implements HORSE-02.
func (s *Server) GetHorsePublicDetail(c *gin.Context, horseId generated.HorseIdPath) {
	if s.horse == nil {
		respondNotImplemented(c)
		return
	}
	s.horse.GetHorsePublicDetail(c, horseId)
}
