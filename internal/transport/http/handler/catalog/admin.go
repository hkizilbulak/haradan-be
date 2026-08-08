package catalog

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

func (h *Handler) requireAdminBO(c *gin.Context) bool {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return false
	}
	if err := authz.RequireAdminBO(p); err != nil {
		h.respond(c, h.logger, err)
		return false
	}
	return true
}
func (h *Handler) ListCategoriesAdmin(c *gin.Context, q generated.ListCategoriesAdminParams) {
	if !h.requireAdminBO(c) {
		return
	}
	limit := 50
	if q.Limit != nil {
		limit = int(*q.Limit)
	}
	items, err := h.svc.ListCategoriesAdmin(c.Request.Context(), q.IsActive, limit+1)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	c.JSON(http.StatusOK, generated.AdminCategoryListResponse{Items: mapCategories(items), HasMore: hasMore})
}
func (h *Handler) CreateCategory(c *gin.Context) {
	if !h.requireAdminBO(c) {
		return
	}
	var req generated.CreateCategoryRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	var parent *uuid.UUID
	if req.ParentId != nil {
		v := uuid.UUID(*req.ParentId)
		parent = &v
	}
	out, err := h.svc.CreateCategory(c.Request.Context(), domaincatalog.Category{ParentID: parent, Slug: req.Slug, Name: req.Name, Description: req.Description}, req.SortOrder)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusCreated, mapCategory(out))
}
func (h *Handler) GetCategoryAdminDetail(c *gin.Context, id generated.CategoryIdPath) {
	if !h.requireAdminBO(c) {
		return
	}
	out, err := h.svc.GetCategoryAdminDetail(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapCategory(out))
}
func (h *Handler) UpdateCategory(c *gin.Context, id generated.CategoryIdPath) {
	if !h.requireAdminBO(c) {
		return
	}
	raw, err := bind.PatchObject(c, categoryPatchKeys)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	expected, err := bind.RequireExpectedVersion(raw)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	p, err := decodeCategoryPatch(raw)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out, err := h.svc.UpdateCategory(c.Request.Context(), uuid.UUID(id), p, expected)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapCategory(out))
}
func (h *Handler) SetCategoryActive(c *gin.Context, id generated.CategoryIdPath) {
	if !h.requireAdminBO(c) {
		return
	}
	var req generated.SetActiveRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, err := h.svc.SetCategoryActive(c.Request.Context(), uuid.UUID(id), req.IsActive, req.ExpectedVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapCategory(out))
}
func (h *Handler) ReparentCategory(c *gin.Context, id generated.CategoryIdPath) {
	if !h.requireAdminBO(c) {
		return
	}
	var req generated.ReparentCategoryRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	var parent *uuid.UUID
	if req.NewParentId != nil {
		v := uuid.UUID(*req.NewParentId)
		parent = &v
	}
	out, err := h.svc.ReparentCategory(c.Request.Context(), uuid.UUID(id), parent, req.ExpectedVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapCategory(out))
}
func (h *Handler) ReorderCategories(c *gin.Context) {
	if !h.requireAdminBO(c) {
		return
	}
	var req generated.ReorderCategoriesRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	if err := h.svc.ReorderCategories(c.Request.Context(), mapReorder(req.Items)); err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) ListCategoryPropertiesAdmin(c *gin.Context, cid generated.CategoryIdPath) {
	if !h.requireAdminBO(c) {
		return
	}
	items, err := h.svc.ListCategoryPropertiesAdmin(c.Request.Context(), uuid.UUID(cid))
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out, err := mapAdminProperties(items)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.AdminCategoryPropertyListResponse{Items: out})
}
func (h *Handler) CreateCategoryProperty(c *gin.Context, cid generated.CategoryIdPath) {
	if !h.requireAdminBO(c) {
		return
	}
	var req generated.CreateCategoryPropertyRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	p := propertyFromCreate(uuid.UUID(cid), req)
	out, err := h.svc.CreateCategoryProperty(c.Request.Context(), p, req.SortOrder)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	mapped, err := mapAdminProperty(out)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusCreated, mapped)
}
func (h *Handler) UpdateCategoryProperty(c *gin.Context, cid generated.CategoryIdPath, pid generated.PropertyIdPath) {
	if !h.requireAdminBO(c) {
		return
	}
	raw, err := bind.PatchObject(c, propertyPatchKeys)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	expected, err := bind.RequireExpectedVersion(raw)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	p, err := decodePropertyPatch(raw)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out, err := h.svc.UpdateCategoryProperty(c.Request.Context(), uuid.UUID(cid), uuid.UUID(pid), p, expected)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	mapped, err := mapAdminProperty(out)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapped)
}
func (h *Handler) SetCategoryPropertyActive(c *gin.Context, cid generated.CategoryIdPath, pid generated.PropertyIdPath) {
	if !h.requireAdminBO(c) {
		return
	}
	var req generated.SetActiveRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, err := h.svc.SetCategoryPropertyActive(c.Request.Context(), uuid.UUID(cid), uuid.UUID(pid), req.IsActive, req.ExpectedVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	mapped, err := mapAdminProperty(out)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapped)
}
func (h *Handler) ReorderCategoryProperties(c *gin.Context, cid generated.CategoryIdPath) {
	if !h.requireAdminBO(c) {
		return
	}
	var req generated.ReorderCategoryPropertiesRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	if err := h.svc.ReorderCategoryProperties(c.Request.Context(), uuid.UUID(cid), mapReorder(req.Items)); err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.Status(http.StatusNoContent)
}

var categoryPatchKeys = map[string]struct{}{"expectedVersion": {}, "slug": {}, "name": {}, "description": {}, "sortOrder": {}}
var propertyPatchKeys = map[string]struct{}{"expectedVersion": {}, "title": {}, "helpText": {}, "isRequired": {}, "isPublicVisible": {}, "isFormVisible": {}, "isFilterable": {}, "sortOrder": {}, "options": {}, "validation": {}, "defaultValue": {}, "uiMetadata": {}}

func decodeCategoryPatch(raw map[string]json.RawMessage) (domaincatalog.CategoryPatch, error) {
	var p domaincatalog.CategoryPatch
	for k, v := range raw {
		switch k {
		case "slug":
			p.SlugSet = true
			if err := json.Unmarshal(v, &p.Slug); err != nil {
				return p, bindErr()
			}
		case "name":
			p.NameSet = true
			if err := json.Unmarshal(v, &p.Name); err != nil {
				return p, bindErr()
			}
		case "description":
			p.DescriptionSet = true
			if !bind.IsJSONNull(v) {
				var x string
				if err := json.Unmarshal(v, &x); err != nil {
					return p, bindErr()
				}
				p.Description = &x
			}
		case "sortOrder":
			p.SortOrderSet = true
			if err := json.Unmarshal(v, &p.SortOrder); err != nil || p.SortOrder < 0 {
				return p, bindErr()
			}
		}
	}
	return p, nil
}
func decodePropertyPatch(raw map[string]json.RawMessage) (domaincatalog.PropertyPatch, error) {
	var p domaincatalog.PropertyPatch
	var err error
	for k, v := range raw {
		switch k {
		case "title":
			p.TitleSet = true
			err = json.Unmarshal(v, &p.Title)
		case "helpText":
			p.HelpTextSet = true
			if !bind.IsJSONNull(v) {
				var x string
				err = json.Unmarshal(v, &x)
				p.HelpText = &x
			}
		case "isRequired":
			p.IsRequiredSet = true
			err = json.Unmarshal(v, &p.IsRequired)
		case "isPublicVisible":
			p.IsPublicVisibleSet = true
			err = json.Unmarshal(v, &p.IsPublicVisible)
		case "isFormVisible":
			p.IsFormVisibleSet = true
			err = json.Unmarshal(v, &p.IsFormVisible)
		case "isFilterable":
			p.IsFilterableSet = true
			err = json.Unmarshal(v, &p.IsFilterable)
		case "sortOrder":
			p.SortOrderSet = true
			err = json.Unmarshal(v, &p.SortOrder)
		case "options":
			p.OptionsSet = true
			p.Options = v
		case "validation":
			p.ValidationSet = true
			p.Validation = v
		case "defaultValue":
			p.DefaultValueSet = true
			p.DefaultValue = v
		case "uiMetadata":
			p.UIMetadataSet = true
			p.UIMetadata = v
		}
		if err != nil || p.SortOrder < 0 {
			return p, bindErr()
		}
	}
	return p, nil
}
func bindErr() error { return apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage) }
func mapCategories(v []domaincatalog.Category) []generated.AdminCategoryDetailResponse {
	out := make([]generated.AdminCategoryDetailResponse, 0, len(v))
	for _, x := range v {
		out = append(out, mapCategory(x))
	}
	return out
}
func mapCategory(x domaincatalog.Category) generated.AdminCategoryDetailResponse {
	var parent *openapi_types.UUID
	if x.ParentID != nil {
		v := openapi_types.UUID(*x.ParentID)
		parent = &v
	}
	return generated.AdminCategoryDetailResponse{Id: openapi_types.UUID(x.ID), ParentId: parent, Slug: x.Slug, Name: x.Name, Description: x.Description, IsActive: x.IsActive, SortOrder: x.SortOrder, Version: x.Version}
}
func mapReorder(v []generated.ReorderItem) []domaincatalog.ReorderItem {
	out := make([]domaincatalog.ReorderItem, 0, len(v))
	for _, x := range v {
		out = append(out, domaincatalog.ReorderItem{ID: uuid.UUID(x.Id), ExpectedVersion: x.ExpectedVersion, SortOrder: x.SortOrder})
	}
	return out
}
func propertyFromCreate(cid uuid.UUID, r generated.CreateCategoryPropertyRequest) domaincatalog.Property {
	code := ""
	if r.Code != nil {
		code = strings.TrimSpace(*r.Code)
	}
	p := domaincatalog.Property{CategoryID: cid, Code: code, Title: r.Title, HelpText: r.HelpText, DataType: string(r.DataType), Options: json.RawMessage(`[]`), Validation: json.RawMessage(`{}`), UIMetadata: json.RawMessage(`{}`)}
	if r.IsRequired != nil {
		p.IsRequired = *r.IsRequired
	}
	if r.IsPublicVisible != nil {
		p.IsPublicVisible = *r.IsPublicVisible
	} else {
		p.IsPublicVisible = true
	}
	if r.IsFormVisible != nil {
		p.IsFormVisible = *r.IsFormVisible
	} else {
		p.IsFormVisible = true
	}
	if r.IsFilterable != nil {
		p.IsFilterable = *r.IsFilterable
	}
	if r.Options != nil {
		p.Options, _ = json.Marshal(*r.Options)
	}
	if r.Validation != nil {
		p.Validation, _ = json.Marshal(*r.Validation)
	}
	if r.UiMetadata != nil {
		p.UIMetadata, _ = json.Marshal(*r.UiMetadata)
	}
	if r.DefaultValue != nil {
		p.DefaultValue, _ = json.Marshal(r.DefaultValue)
	}
	return p
}
func mapAdminProperties(v []domaincatalog.Property) ([]generated.AdminCategoryPropertyResponse, error) {
	out := make([]generated.AdminCategoryPropertyResponse, 0, len(v))
	for _, x := range v {
		m, e := mapAdminProperty(x)
		if e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, nil
}
func mapAdminProperty(p domaincatalog.Property) (generated.AdminCategoryPropertyResponse, error) {
	opts, err := decodeObjectArray(p.Options)
	if err != nil {
		return generated.AdminCategoryPropertyResponse{}, apperr.Internal(err)
	}
	var validation, ui map[string]interface{}
	if json.Unmarshal(p.Validation, &validation) != nil || json.Unmarshal(p.UIMetadata, &ui) != nil {
		return generated.AdminCategoryPropertyResponse{}, apperr.Internal(errInvalidPropertyDataType)
	}
	out := generated.AdminCategoryPropertyResponse{Id: openapi_types.UUID(p.ID), CategoryId: openapi_types.UUID(p.CategoryID), Code: p.Code, Title: p.Title, HelpText: p.HelpText, DataType: generated.PropertyDataType(p.DataType), IsRequired: p.IsRequired, IsPublicVisible: p.IsPublicVisible, IsFormVisible: p.IsFormVisible, IsFilterable: p.IsFilterable, SortOrder: p.SortOrder, IsActive: p.IsActive, Version: p.Version, Options: opts, Validation: validation, UiMetadata: ui}
	if len(p.DefaultValue) > 0 && string(p.DefaultValue) != "null" {
		_ = json.Unmarshal(p.DefaultValue, &out.DefaultValue)
	}
	return out, nil
}
