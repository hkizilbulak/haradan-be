package catalog

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	appcatalog "github.com/hkizilbulak/haradan-be/internal/application/catalog"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

var errInvalidPropertyDataType = errors.New("invalid property data type")

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes Catalog OpenAPI operations.
type Handler struct {
	svc     *appcatalog.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs a Catalog HTTP handler.
func NewHandler(svc *appcatalog.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// GetPublicCategoryTree handles GET /v1/categories.
func (h *Handler) GetPublicCategoryTree(c *gin.Context) {
	items, err := h.svc.GetPublicCategoryTree(c.Request.Context())
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.CategoryTreeResponse{Items: mapTree(items)})
}

// GetCategoryFormDefinition handles GET /v1/categories/{categoryId}/form.
func (h *Handler) GetCategoryFormDefinition(c *gin.Context, categoryID generated.CategoryIdPath) {
	def, err := h.svc.GetCategoryFormDefinition(c.Request.Context(), uuid.UUID(categoryID))
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	props, err := mapProperties(def.Properties)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.CategoryFormDefinitionResponse{
		CategoryId: openapi_types.UUID(def.Category.ID),
		Slug:       def.Category.Slug,
		Name:       def.Category.Name,
		Properties: props,
	})
}

func mapTree(nodes []appcatalog.TreeNode) []generated.CategoryTreeNode {
	out := make([]generated.CategoryTreeNode, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, generated.CategoryTreeNode{
			Id:       openapi_types.UUID(n.ID),
			Slug:     n.Slug,
			Name:     n.Name,
			Children: mapTree(n.Children),
		})
	}
	return out
}

func mapProperties(props []domaincatalog.Property) ([]generated.CategoryPropertyPublic, error) {
	out := make([]generated.CategoryPropertyPublic, 0, len(props))
	for _, p := range props {
		mapped, err := mapProperty(p)
		if err != nil {
			return nil, err
		}
		out = append(out, mapped)
	}
	return out, nil
}

func mapProperty(p domaincatalog.Property) (generated.CategoryPropertyPublic, error) {
	dataType := generated.PropertyDataType(p.DataType)
	if !dataType.Valid() {
		return generated.CategoryPropertyPublic{}, apperr.Internal(errInvalidPropertyDataType)
	}

	options, err := decodeObjectArray(p.Options)
	if err != nil {
		return generated.CategoryPropertyPublic{}, apperr.Internal(err)
	}

	item := generated.CategoryPropertyPublic{
		Code:         p.Code,
		Title:        p.Title,
		HelpText:     p.HelpText,
		DataType:     dataType,
		IsRequired:   p.IsRequired,
		IsFilterable: p.IsFilterable,
		SortOrder:    p.SortOrder,
		Options:      options,
	}

	if len(p.DefaultValue) > 0 && string(p.DefaultValue) != "null" {
		var dv interface{}
		if err := json.Unmarshal(p.DefaultValue, &dv); err != nil {
			return generated.CategoryPropertyPublic{}, apperr.Internal(err)
		}
		item.DefaultValue = dv
	}

	if len(p.UIMetadata) > 0 {
		var ui map[string]interface{}
		if err := json.Unmarshal(p.UIMetadata, &ui); err != nil {
			return generated.CategoryPropertyPublic{}, apperr.Internal(err)
		}
		item.UiMetadata = &ui
	}

	return item, nil
}

func decodeObjectArray(raw json.RawMessage) ([]map[string]interface{}, error) {
	if len(raw) == 0 {
		return []map[string]interface{}{}, nil
	}
	var out []map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return []map[string]interface{}{}, nil
	}
	return out, nil
}
