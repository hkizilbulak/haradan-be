// Package advert exposes the ADVERT-OWNER-01..11 OpenAPI operations.
package advert

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appadvert "github.com/hkizilbulak/haradan-be/internal/application/advert"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

const malformedBodyMessage = "İstek gövdesi geçersiz."

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes the owner-scoped advert OpenAPI operations.
type Handler struct {
	svc     *appadvert.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs an advert owner HTTP handler.
func NewHandler(svc *appadvert.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// Service exposes the shared application service for root-level public routes.
func (h *Handler) Service() *appadvert.Service { return h.svc }

// CreateAdvertDraft handles POST /v1/me/adverts.
func (h *Handler) CreateAdvertDraft(c *gin.Context) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	var req generated.CreateAdvertDraftRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	in := appadvert.CreateDraftInput{
		CategoryID:  req.CategoryId,
		DistrictID:  req.DistrictId,
		HorseID:     req.HorseId,
		Title:       req.Title,
		Description: req.Description,
		Price:       moneyInput(req.Price),
	}
	out, err := h.svc.CreateAdvertDraft(c.Request.Context(), ownerID, in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusCreated, mapOwnerView(out))
}

// ListMyAdverts handles GET /v1/me/adverts.
func (h *Handler) ListMyAdverts(c *gin.Context, params generated.ListMyAdvertsParams) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	in := appadvert.ListInput{Cursor: params.Cursor, Limit: params.Limit}
	if params.Status != nil {
		s := string(*params.Status)
		in.Status = &s
	}
	out, err := h.svc.ListMyAdverts(c.Request.Context(), ownerID, in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	items := make([]ownerAdvertJSON, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, mapOwnerView(item))
	}
	c.JSON(http.StatusOK, gin.H{
		"items":      items,
		"hasMore":    out.HasMore,
		"nextCursor": out.NextCursor,
	})
}

// GetMyAdvert handles GET /v1/me/adverts/{advertId}.
func (h *Handler) GetMyAdvert(c *gin.Context, advertID generated.AdvertIdPath) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	out, err := h.svc.GetMyAdvert(c.Request.Context(), ownerID, advertID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerView(out))
}

// UpdateAdvertDraftDetails handles PATCH /v1/me/adverts/{advertId}.
func (h *Handler) UpdateAdvertDraftDetails(c *gin.Context, advertID generated.AdvertIdPath) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	in, err := decodeUpdateDetailsInput(c)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out, err := h.svc.UpdateAdvertDraftDetails(c.Request.Context(), ownerID, advertID, in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerView(out))
}

// ChangeAdvertDraftCategory handles PUT /v1/me/adverts/{advertId}/category.
func (h *Handler) ChangeAdvertDraftCategory(c *gin.Context, advertID generated.AdvertIdPath) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	var req generated.ChangeAdvertDraftCategoryRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	in := appadvert.ChangeCategoryInput{
		ExpectedVersion: req.ExpectedVersion,
		CategoryID:      req.CategoryId,
	}
	out, err := h.svc.ChangeAdvertDraftCategory(c.Request.Context(), ownerID, advertID, in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerView(out))
}

// ReplaceAdvertDynamicProperties handles PUT /v1/me/adverts/{advertId}/properties.
func (h *Handler) ReplaceAdvertDynamicProperties(c *gin.Context, advertID generated.AdvertIdPath) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	var req generated.ReplaceAdvertDynamicPropertiesRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	props, err := json.Marshal(req.Properties)
	if err != nil {
		h.respond(c, h.logger, apperr.BadRequest(apperr.CodeValidation, malformedBodyMessage))
		return
	}
	in := appadvert.ReplacePropertiesInput{
		ExpectedVersion: req.ExpectedVersion,
		Properties:      props,
	}
	out, err := h.svc.ReplaceAdvertDynamicProperties(c.Request.Context(), ownerID, advertID, in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerView(out))
}

// SubmitAdvertForReview handles POST /v1/me/adverts/{advertId}/submit.
func (h *Handler) SubmitAdvertForReview(c *gin.Context, advertID generated.AdvertIdPath) {
	ownerID, expectedVersion, ok := h.principalAndExpectedVersion(c)
	if !ok {
		return
	}
	out, err := h.svc.SubmitAdvertForReview(c.Request.Context(), ownerID, advertID, expectedVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerView(out))
}

// ResubmitAdvertForReview handles POST /v1/me/adverts/{advertId}/resubmit.
func (h *Handler) ResubmitAdvertForReview(c *gin.Context, advertID generated.AdvertIdPath) {
	ownerID, expectedVersion, ok := h.principalAndExpectedVersion(c)
	if !ok {
		return
	}
	out, err := h.svc.ResubmitAdvertForReview(c.Request.Context(), ownerID, advertID, expectedVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerView(out))
}

// SoftDeleteAdvertDraft handles DELETE /v1/me/adverts/{advertId}.
func (h *Handler) SoftDeleteAdvertDraft(c *gin.Context, advertID generated.AdvertIdPath, params generated.SoftDeleteAdvertDraftParams) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	out, err := h.svc.SoftDeleteAdvertDraft(c.Request.Context(), ownerID, advertID, params.ExpectedVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerView(out))
}

// MarkAdvertSold handles POST /v1/me/adverts/{advertId}/sold.
func (h *Handler) MarkAdvertSold(c *gin.Context, advertID generated.AdvertIdPath) {
	ownerID, expectedVersion, ok := h.principalAndExpectedVersion(c)
	if !ok {
		return
	}
	out, err := h.svc.MarkAdvertSold(c.Request.Context(), ownerID, advertID, expectedVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerView(out))
}

// ArchiveAdvert handles POST /v1/me/adverts/{advertId}/archive.
func (h *Handler) ArchiveAdvert(c *gin.Context, advertID generated.AdvertIdPath) {
	ownerID, expectedVersion, ok := h.principalAndExpectedVersion(c)
	if !ok {
		return
	}
	out, err := h.svc.ArchiveAdvert(c.Request.Context(), ownerID, advertID, expectedVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerView(out))
}

func (h *Handler) requirePrincipal(c *gin.Context) (uuid.UUID, bool) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return uuid.Nil, false
	}
	return p.UserID, true
}

func (h *Handler) principalAndExpectedVersion(c *gin.Context) (uuid.UUID, int, bool) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return uuid.Nil, 0, false
	}
	var req generated.ExpectedVersionRequest
	if !bind.JSONBody(c, &req) {
		return uuid.Nil, 0, false
	}
	return ownerID, req.ExpectedVersion, true
}

func moneyInput(m *generated.Money) *appadvert.MoneyInput {
	if m == nil {
		return nil
	}
	amount := int64(m.AmountMinor)
	currency := m.Currency
	return &appadvert.MoneyInput{AmountMinor: &amount, Currency: &currency}
}

func moneyResponse(m *domainadvert.Money) *generated.Money {
	if m == nil {
		return nil
	}
	return &generated.Money{AmountMinor: int(m.AmountMinor), Currency: m.Currency}
}

type ownerAdvertJSON struct {
	generated.OwnerAdvertResponse
	ProvinceId *uuid.UUID `json:"provinceId"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	SoldAt     *time.Time `json:"soldAt,omitempty"`
}

func mapOwnerAdvertBase(v domainadvert.OwnerView) generated.OwnerAdvertResponse {
	props := map[string]interface{}{}
	if len(v.Properties) > 0 {
		_ = json.Unmarshal(v.Properties, &props)
		if props == nil {
			props = map[string]interface{}{}
		}
	}
	media := make([]generated.OwnerMediaRelationItem, 0, len(v.Media))
	for _, m := range v.Media {
		media = append(media, generated.OwnerMediaRelationItem{
			AssetId:         m.AssetID,
			DisplayOrder:    m.DisplayOrder,
			IsCover:         m.IsCover,
			LifecycleStatus: generated.MediaAssetLifecycle(m.LifecycleStatus),
		})
	}
	return generated.OwnerAdvertResponse{
		Id:                     v.ID,
		Status:                 generated.AdvertStatus(v.Status),
		Version:                v.Version,
		MediaVersion:           v.MediaVersion,
		CategoryId:             v.CategoryID,
		DistrictId:             v.DistrictID,
		HorseId:                v.HorseID,
		Title:                  v.Title,
		Description:            v.Description,
		Price:                  moneyResponse(v.Price),
		Properties:             props,
		Media:                  media,
		PublishedAt:            v.PublishedAt,
		DeletedAt:              v.DeletedAt,
		CategoryClearedWarning: v.CategoryClearedWarning,
	}
}

func mapOwnerView(v domainadvert.OwnerView) ownerAdvertJSON {
	// provinceId + updatedAt + soldAt are owner-list card fields used by FE;
	// kept as additive JSON so OpenAPI regen is not required for this read path.
	return ownerAdvertJSON{
		OwnerAdvertResponse: mapOwnerAdvertBase(v),
		ProvinceId:          v.ProvinceID,
		UpdatedAt:           v.UpdatedAt,
		SoldAt:              v.SoldAt,
	}
}

// decodeUpdateDetailsInput custom-decodes the PATCH body so an explicit JSON
// null (clear the field) is distinguishable from an absent key (leave as is).
// The generated request type's omitempty tags cannot express that distinction.
func decodeUpdateDetailsInput(c *gin.Context) (appadvert.UpdateDetailsInput, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return appadvert.UpdateDetailsInput{}, apperr.BadRequest(apperr.CodeValidation, malformedBodyMessage)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return appadvert.UpdateDetailsInput{}, apperr.BadRequest(apperr.CodeValidation, malformedBodyMessage)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF && err != nil {
		return appadvert.UpdateDetailsInput{}, apperr.BadRequest(apperr.CodeValidation, malformedBodyMessage)
	}

	evRaw, ok := raw["expectedVersion"]
	if !ok {
		return appadvert.UpdateDetailsInput{}, apperr.BadRequest(apperr.CodeValidation, malformedBodyMessage)
	}
	var expectedVersion int
	if err := json.Unmarshal(evRaw, &expectedVersion); err != nil {
		return appadvert.UpdateDetailsInput{}, apperr.BadRequest(apperr.CodeValidation, malformedBodyMessage)
	}

	in := appadvert.UpdateDetailsInput{ExpectedVersion: expectedVersion}

	if v, ok := raw["districtId"]; ok {
		in.DistrictIDSet = true
		if !isJSONNull(v) {
			var id uuid.UUID
			if err := json.Unmarshal(v, &id); err != nil {
				return appadvert.UpdateDetailsInput{}, apperr.BadRequest(apperr.CodeValidation, malformedBodyMessage)
			}
			in.DistrictID = &id
		}
	}
	if v, ok := raw["horseId"]; ok {
		in.HorseIDSet = true
		if !isJSONNull(v) {
			var id uuid.UUID
			if err := json.Unmarshal(v, &id); err != nil {
				return appadvert.UpdateDetailsInput{}, apperr.BadRequest(apperr.CodeValidation, malformedBodyMessage)
			}
			in.HorseID = &id
		}
	}
	if v, ok := raw["title"]; ok {
		in.TitleSet = true
		if !isJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appadvert.UpdateDetailsInput{}, apperr.BadRequest(apperr.CodeValidation, malformedBodyMessage)
			}
			in.Title = &s
		}
	}
	if v, ok := raw["description"]; ok {
		in.DescriptionSet = true
		if !isJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appadvert.UpdateDetailsInput{}, apperr.BadRequest(apperr.CodeValidation, malformedBodyMessage)
			}
			in.Description = &s
		}
	}
	if v, ok := raw["address"]; ok {
		in.AddressSet = true
		if !isJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appadvert.UpdateDetailsInput{}, apperr.BadRequest(apperr.CodeValidation, malformedBodyMessage)
			}
			in.Address = &s
		}
	}
	if v, ok := raw["price"]; ok {
		in.PriceSet = true
		if !isJSONNull(v) {
			var m generated.Money
			if err := json.Unmarshal(v, &m); err != nil {
				return appadvert.UpdateDetailsInput{}, apperr.BadRequest(apperr.CodeValidation, malformedBodyMessage)
			}
			in.Price = moneyInput(&m)
		}
	}
	return in, nil
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
