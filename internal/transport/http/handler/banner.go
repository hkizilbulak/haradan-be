package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

func (s *Server) ListBannersAdmin(c *gin.Context, p generated.ListBannersAdminParams) {
	if s.banner == nil {
		respondNotImplemented(c)
		return
	}
	s.banner.ListBannersAdmin(c, p)
}
func (s *Server) CreateBanner(c *gin.Context) {
	if s.banner == nil {
		respondNotImplemented(c)
		return
	}
	s.banner.CreateBanner(c)
}
func (s *Server) ReorderBanners(c *gin.Context) {
	if s.banner == nil {
		respondNotImplemented(c)
		return
	}
	s.banner.ReorderBanners(c)
}
func (s *Server) GetBannerAdminDetail(c *gin.Context, id generated.BannerIdPath) {
	if s.banner == nil {
		respondNotImplemented(c)
		return
	}
	s.banner.GetBannerAdminDetail(c, id)
}
func (s *Server) UpdateBanner(c *gin.Context, id generated.BannerIdPath) {
	if s.banner == nil {
		respondNotImplemented(c)
		return
	}
	s.banner.UpdateBanner(c, id)
}
func (s *Server) SetBannerStatus(c *gin.Context, id generated.BannerIdPath) {
	if s.banner == nil {
		respondNotImplemented(c)
		return
	}
	s.banner.SetBannerStatus(c, id)
}
func (s *Server) ListActiveBannersByPlacement(c *gin.Context, p generated.ListActiveBannersByPlacementParams) {
	if s.banner == nil {
		respondNotImplemented(c)
		return
	}
	s.banner.ListActiveBannersByPlacement(c, p)
}
