package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// ListActiveProvinces implements GEO-01.
func (s *Server) ListActiveProvinces(c *gin.Context) {
	if s.geo == nil {
		respondNotImplemented(c)
		return
	}
	s.geo.ListActiveProvinces(c)
}

// SearchProvinces implements GEO-02.
func (s *Server) SearchProvinces(c *gin.Context, params generated.SearchProvincesParams) {
	if s.geo == nil {
		respondNotImplemented(c)
		return
	}
	s.geo.SearchProvinces(c, params)
}

// ListDistrictsByProvince implements GEO-03.
func (s *Server) ListDistrictsByProvince(c *gin.Context, provinceId generated.ProvinceIdPath) {
	if s.geo == nil {
		respondNotImplemented(c)
		return
	}
	s.geo.ListDistrictsByProvince(c, provinceId)
}

// SearchDistricts implements GEO-04.
func (s *Server) SearchDistricts(c *gin.Context, params generated.SearchDistrictsParams) {
	if s.geo == nil {
		respondNotImplemented(c)
		return
	}
	s.geo.SearchDistricts(c, params)
}
