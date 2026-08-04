package horse

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	apphorse "github.com/hkizilbulak/haradan-be/internal/application/horse"
	domainhorse "github.com/hkizilbulak/haradan-be/internal/domain/horse"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes Horse OpenAPI operations.
type Handler struct {
	svc     *apphorse.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs a Horse HTTP handler.
func NewHandler(svc *apphorse.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// SearchHorsesForSelection handles GET /v1/horses.
func (h *Handler) SearchHorsesForSelection(c *gin.Context, params generated.SearchHorsesForSelectionParams) {
	var limit *int
	if params.Limit != nil {
		v := int(*params.Limit)
		limit = &v
	}
	items, err := h.svc.SearchForSelection(c.Request.Context(), params.Q, params.TjkNumber, limit)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.HorseSelectionListResponse{Items: mapSelection(items)})
}

// GetHorsePublicDetail handles GET /v1/horses/{horseId}.
func (h *Handler) GetHorsePublicDetail(c *gin.Context, horseID uuid.UUID) {
	out, err := h.svc.GetPublicDetail(c.Request.Context(), horseID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapPublicDetail(out))
}

func mapSelection(items []domainhorse.SelectionProjection) []generated.HorseSelectionItem {
	out := make([]generated.HorseSelectionItem, 0, len(items))
	for _, item := range items {
		out = append(out, generated.HorseSelectionItem{
			Id:           openapi_types.UUID(item.ID),
			OriginalName: item.OriginalName,
			TjkNumber:    item.TJKNumber,
			BirthYear:    item.BirthYear,
			SireName:     item.SireName,
			DamName:      item.DamName,
		})
	}
	return out
}

func mapPublicDetail(out domainhorse.PublicDetail) generated.HorsePublicDetailResponse {
	detail := map[string]interface{}{}
	if len(out.Detail) > 0 {
		_ = json.Unmarshal(out.Detail, &detail)
		if detail == nil {
			detail = map[string]interface{}{}
		}
	}
	return generated.HorsePublicDetailResponse{
		Id:           openapi_types.UUID(out.ID),
		OriginalName: out.OriginalName,
		TjkNumber:    out.TJKNumber,
		BirthYear:    out.BirthYear,
		SireName:     out.SireName,
		DamName:      out.DamName,
		Breed:        out.Breed,
		Gender:       out.Gender,
		Coat:         out.Coat,
		Detail:       detail,
	}
}
