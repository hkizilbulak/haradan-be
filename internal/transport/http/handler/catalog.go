package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

func (s *Server) ListCategoriesAdmin(c *gin.Context, p generated.ListCategoriesAdminParams) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.ListCategoriesAdmin(c, p)
}
func (s *Server) CreateCategory(c *gin.Context) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.CreateCategory(c)
}
func (s *Server) ReorderCategories(c *gin.Context) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.ReorderCategories(c)
}
func (s *Server) GetCategoryAdminDetail(c *gin.Context, id generated.CategoryIdPath) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.GetCategoryAdminDetail(c, id)
}
func (s *Server) UpdateCategory(c *gin.Context, id generated.CategoryIdPath) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.UpdateCategory(c, id)
}
func (s *Server) SetCategoryActive(c *gin.Context, id generated.CategoryIdPath) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.SetCategoryActive(c, id)
}
func (s *Server) DeleteCategoryAdmin(c *gin.Context, id generated.CategoryIdPath) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.DeleteCategoryAdmin(c, id)
}
func (s *Server) ListCategoryPropertiesAdmin(c *gin.Context, id generated.CategoryIdPath) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.ListCategoryPropertiesAdmin(c, id)
}
func (s *Server) CreateCategoryProperty(c *gin.Context, id generated.CategoryIdPath) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.CreateCategoryProperty(c, id)
}
func (s *Server) ReorderCategoryProperties(c *gin.Context, id generated.CategoryIdPath) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.ReorderCategoryProperties(c, id)
}
func (s *Server) UpdateCategoryProperty(c *gin.Context, id generated.CategoryIdPath, pid generated.PropertyIdPath) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.UpdateCategoryProperty(c, id, pid)
}
func (s *Server) SetCategoryPropertyActive(c *gin.Context, id generated.CategoryIdPath, pid generated.PropertyIdPath) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.SetCategoryPropertyActive(c, id, pid)
}
func (s *Server) DeleteCategoryPropertyAdmin(c *gin.Context, id generated.CategoryIdPath, pid generated.PropertyIdPath) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.DeleteCategoryPropertyAdmin(c, id, pid)
}
func (s *Server) ReparentCategory(c *gin.Context, id generated.CategoryIdPath) {
	if s.catalog == nil {
		respondNotImplemented(c)
		return
	}
	s.catalog.ReparentCategory(c, id)
}

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
