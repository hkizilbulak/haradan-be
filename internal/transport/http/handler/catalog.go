package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// GetPublicCategoryTree implements CATALOG-01.
func (s *Server) GetPublicCategoryTree(c *gin.Context) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.GetPublicCategoryTree(c)
}

// GetCategoryFormDefinition implements CATALOG-02.
func (s *Server) GetCategoryFormDefinition(c *gin.Context, categoryId generated.CategoryIdPath) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.GetCategoryFormDefinition(c, categoryId)
}
