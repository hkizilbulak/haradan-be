package packaging

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes packaging admin and URGENT OpenAPI operations.
type Handler struct {
	svc     *apppackaging.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs a packaging HTTP handler.
func NewHandler(svc *apppackaging.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// ListPublicPackages handles GET /v1/packages and deliberately exposes only
// active, buyer-facing package fields.
func (h *Handler) ListPublicPackages(c *gin.Context) {
	items, err := h.svc.ListPackages(c.Request.Context(), false)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out := make([]generated.PublicPackage, 0, len(items))
	for _, item := range items {
		out = append(out, mapPublicPackage(item))
	}
	c.JSON(http.StatusOK, generated.PublicPackageListResponse{Items: out})
}

// CreateAdminPackage handles POST /v1/admin/packages.
func (h *Handler) CreateAdminPackage(c *gin.Context) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	var req generated.CreatePackageRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	var price *int64
	currency := ""
	if req.CurrencyCode != nil {
		currency = *req.CurrencyCode
	}
	if req.DisplayPrice != nil {
		amount := int64(req.DisplayPrice.AmountMinor)
		price = &amount
		if currency == "" {
			currency = req.DisplayPrice.Currency
		}
	}
	var code domainpackaging.PackageCode
	if req.Code != nil {
		code = domainpackaging.PackageCode(*req.Code)
	}
	priority := 0
	prioritySet := false
	if req.SearchPriority != nil {
		priority = *req.SearchPriority
		prioritySet = true
	}
	out, err := h.svc.CreatePackage(c.Request.Context(), apppackaging.CreatePackageInput{
		ActorUserID:             actorID,
		Code:                    code,
		DisplayName:             req.DisplayName,
		Description:             req.Description,
		BadgeText:               req.BadgeText,
		Benefits:                req.Benefits,
		DisplayPriceAmountMinor: price,
		CurrencyCode:            currency,
		DefaultDurationDays:     req.DefaultDurationDays,
		AllowsUrgent:            req.AllowsUrgent,
		ShowcaseEligible:        req.ShowcaseEligible,
		SearchPriority:          priority,
		SearchPrioritySet:       prioritySet,
		BroadcastOnPublish:      req.BroadcastOnPublish,
		IsActive:                req.IsActive,
		SortOrder:               req.SortOrder,
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusCreated, mapPackageAdminView(out))
}

// ReorderPackages handles PUT /v1/admin/packages/reorder.
func (h *Handler) ReorderPackages(c *gin.Context) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	var req generated.ReorderPackagesRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	items := make([]apppackaging.ReorderPackageItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, apppackaging.ReorderPackageItem{
			ID:              uuid.UUID(item.Id),
			ExpectedVersion: item.ExpectedVersion,
			SortOrder:       item.SortOrder,
		})
	}
	if err := h.svc.ReorderPackages(c.Request.Context(), actorID, items); err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.SuccessMessageResponse{Message: "Sıralama güncellendi."})
}

// ListAdminPackages handles GET /v1/admin/packages.
func (h *Handler) ListAdminPackages(c *gin.Context) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	items, err := h.svc.ListAdminPackages(c.Request.Context(), actorID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out := make([]generated.PackageAdminView, 0, len(items))
	for _, p := range items {
		out = append(out, mapPackageAdminView(p))
	}
	c.JSON(http.StatusOK, generated.PackageAdminListResponse{Items: out})
}

// GetAdminPackage handles GET /v1/admin/packages/{packageCode}.
func (h *Handler) GetAdminPackage(c *gin.Context, packageCode generated.PackageCodePath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	out, err := h.svc.GetPackageByCode(c.Request.Context(), actorID, domainpackaging.PackageCode(packageCode))
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapPackageAdminView(out))
}

// UpdateAdminPackage handles PATCH /v1/admin/packages/{packageCode}.
func (h *Handler) UpdateAdminPackage(c *gin.Context, packageCode generated.PackageCodePath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	in, err := decodeUpdatePackageInput(c, actorID, domainpackaging.PackageCode(packageCode))
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out, err := h.svc.UpdatePackage(c.Request.Context(), in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapPackageAdminView(out))
}

// GetAdminAdvertPackage handles GET /v1/admin/adverts/{advertId}/package.
func (h *Handler) GetAdminAdvertPackage(c *gin.Context, advertID generated.AdvertIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	out, err := h.svc.GetAdminAdvertPackage(c.Request.Context(), actorID, advertID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapAssignmentView(out))
}

// AssignAdminAdvertPackage handles PUT /v1/admin/adverts/{advertId}/package.
func (h *Handler) AssignAdminAdvertPackage(c *gin.Context, advertID generated.AdvertIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	var req generated.AssignAdvertPackageRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, err := h.svc.AssignAdvertPackage(c.Request.Context(), apppackaging.AssignAdvertPackageInput{
		ActorUserID: actorID,
		AdvertID:    advertID,
		PackageCode: domainpackaging.PackageCode(req.PackageCode),
		StartsAt:    req.StartsAt,
		EndsAt:      req.EndsAt,
		Reason:      req.Reason,
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapAssignmentView(out))
}

// AssignOwnerAdvertPackage handles PUT /v1/me/adverts/{advertId}/package.
func (h *Handler) AssignOwnerAdvertPackage(c *gin.Context, advertID generated.AdvertIdPath) {
	principal, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Oturum gerekli."))
		return
	}
	var req generated.AssignOwnerAdvertPackageRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, err := h.svc.AssignOwnerAdvertPackage(
		c.Request.Context(),
		principal.UserID,
		advertID,
		domainpackaging.PackageCode(req.PackageCode),
	)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapAssignmentView(out))
}

// ListAdminAdvertPackageHistory handles GET /v1/admin/adverts/{advertId}/package-history.
func (h *Handler) ListAdminAdvertPackageHistory(
	c *gin.Context,
	advertID generated.AdvertIdPath,
	params generated.ListAdminAdvertPackageHistoryParams,
) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	out, err := h.svc.ListAdvertPackageHistory(c.Request.Context(), apppackaging.ListAdvertPackageHistoryInput{
		ActorUserID: actorID,
		AdvertID:    advertID,
		Cursor:      params.Cursor,
		Limit:       params.Limit,
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	items := make([]generated.AdvertPackageHistoryItem, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, mapHistoryItem(item))
	}
	c.JSON(http.StatusOK, generated.AdvertPackageHistoryPage{
		Items:      items,
		NextCursor: out.NextCursor,
		HasMore:    out.HasMore,
	})
}

// CancelAdminAdvertPackage handles POST /v1/admin/adverts/{advertId}/package/cancel.
func (h *Handler) CancelAdminAdvertPackage(c *gin.Context, advertID generated.AdvertIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	reason, ok := optionalCancelReason(c)
	if !ok {
		return
	}
	if err := h.svc.CancelAdvertPackage(c.Request.Context(), apppackaging.CancelAdvertPackageInput{
		ActorUserID: actorID,
		AdvertID:    advertID,
		Reason:      reason,
	}); err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ActivateAdvertUrgent handles PUT /v1/adverts/{advertId}/urgent.
func (h *Handler) ActivateAdvertUrgent(c *gin.Context, advertID generated.AdvertIdPath) {
	actorID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	out, err := h.svc.ActivateUrgent(c.Request.Context(), actorID, advertID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapUrgentView(out))
}

// DeactivateAdvertUrgent handles DELETE /v1/adverts/{advertId}/urgent.
func (h *Handler) DeactivateAdvertUrgent(c *gin.Context, advertID generated.AdvertIdPath) {
	actorID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	if err := h.svc.DeactivateUrgent(c.Request.Context(), actorID, advertID); err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) requireAdminBO(c *gin.Context) (uuid.UUID, bool) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return uuid.Nil, false
	}
	if err := authz.RequireAdminBO(p); err != nil {
		h.respond(c, h.logger, err)
		return uuid.Nil, false
	}
	return p.UserID, true
}

func (h *Handler) requirePrincipal(c *gin.Context) (uuid.UUID, bool) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return uuid.Nil, false
	}
	return p.UserID, true
}

func optionalCancelReason(c *gin.Context) (*string, bool) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, generated.ErrorResponse{
			Code:    generated.DomainErrorCodeVALIDATIONERROR,
			Message: "İstek gövdesi geçersiz.",
			TraceId: middleware.RequestIDFromContext(c.Request.Context()),
		})
		return nil, false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, true
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var req generated.CancelAdvertPackageRequest
	if err := dec.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, generated.ErrorResponse{
			Code:    generated.DomainErrorCodeVALIDATIONERROR,
			Message: "İstek gövdesi geçersiz.",
			TraceId: middleware.RequestIDFromContext(c.Request.Context()),
		})
		return nil, false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF && err != nil {
		c.JSON(http.StatusBadRequest, generated.ErrorResponse{
			Code:    generated.DomainErrorCodeVALIDATIONERROR,
			Message: "İstek gövdesi geçersiz.",
			TraceId: middleware.RequestIDFromContext(c.Request.Context()),
		})
		return nil, false
	}
	return req.Reason, true
}
