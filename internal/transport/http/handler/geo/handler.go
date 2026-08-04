package geo

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	appgeo "github.com/hkizilbulak/haradan-be/internal/application/geo"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes Geo OpenAPI operations.
type Handler struct {
	svc     *appgeo.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs a Geo HTTP handler.
func NewHandler(svc *appgeo.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// ListActiveProvinces handles GET /v1/provinces.
func (h *Handler) ListActiveProvinces(c *gin.Context) {
	items, err := h.svc.ListActiveProvinces(c.Request.Context())
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.ProvinceListResponse{Items: mapProvinces(items)})
}

// SearchProvinces handles GET /v1/provinces/search.
func (h *Handler) SearchProvinces(c *gin.Context, params generated.SearchProvincesParams) {
	items, err := h.svc.SearchProvinces(c.Request.Context(), params.Q, params.Limit)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.ProvinceListResponse{Items: mapProvinces(items)})
}

// ListDistrictsByProvince handles GET /v1/provinces/{provinceId}/districts.
func (h *Handler) ListDistrictsByProvince(c *gin.Context, provinceID generated.ProvinceIdPath) {
	items, err := h.svc.ListDistrictsByProvince(c.Request.Context(), uuid.UUID(provinceID))
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.DistrictListResponse{Items: mapDistricts(items)})
}

// SearchDistricts handles GET /v1/districts/search.
func (h *Handler) SearchDistricts(c *gin.Context, params generated.SearchDistrictsParams) {
	var provinceID *uuid.UUID
	if params.ProvinceId != nil {
		id := uuid.UUID(*params.ProvinceId)
		provinceID = &id
	}
	items, err := h.svc.SearchDistricts(c.Request.Context(), params.Q, provinceID, params.Limit)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.DistrictListResponse{Items: mapDistricts(items)})
}

func mapProvinces(items []domaingeo.Province) []generated.Province {
	out := make([]generated.Province, 0, len(items))
	for _, p := range items {
		out = append(out, generated.Province{
			Id:        openapi_types.UUID(p.ID),
			Name:      p.Name,
			SortOrder: p.SortOrder,
		})
	}
	return out
}

func mapDistricts(items []domaingeo.District) []generated.District {
	out := make([]generated.District, 0, len(items))
	for _, d := range items {
		out = append(out, generated.District{
			Id:         openapi_types.UUID(d.ID),
			ProvinceId: openapi_types.UUID(d.ProvinceID),
			Name:       d.Name,
			SortOrder:  d.SortOrder,
		})
	}
	return out
}
