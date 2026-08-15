package advert

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appadvert "github.com/hkizilbulak/haradan-be/internal/application/advert"
	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

// ListAdvertModerationQueue handles GET /v1/admin/adverts/moderation.
func (h *Handler) ListAdvertModerationQueue(c *gin.Context, params generated.ListAdvertModerationQueueParams) {
	if _, ok := h.requireAdminBO(c); !ok {
		return
	}
	var status *string
	if params.Status != nil {
		s := string(*params.Status)
		status = &s
	}
	out, err := h.svc.ListAdvertModerationQueue(c.Request.Context(), appadvert.ModerationListInput{
		Status: status,
		Cursor: params.Cursor,
		Limit:  params.Limit,
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	items := make([]generated.OwnerAdvertResponse, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, mapOwnerAdvertBase(item))
	}
	c.JSON(http.StatusOK, generated.ModerationQueueResponse{
		Items:      items,
		NextCursor: out.NextCursor,
		HasMore:    out.HasMore,
	})
}

// GetAdvertModerationDetail handles GET /v1/admin/adverts/{advertId}.
func (h *Handler) GetAdvertModerationDetail(c *gin.Context, advertID generated.AdvertIdPath) {
	if _, ok := h.requireAdminBO(c); !ok {
		return
	}
	out, err := h.svc.GetAdvertModerationDetail(c.Request.Context(), advertID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapModerationDetail(out))
}

// ApproveAdvert handles POST /v1/admin/adverts/{advertId}/approve.
func (h *Handler) ApproveAdvert(c *gin.Context, advertID generated.AdvertIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	var req generated.ExpectedVersionRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, err := h.svc.ApproveAdvert(c.Request.Context(), actorID, advertID, req.ExpectedVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapModerationDetail(out))
}

// RequestAdvertChanges handles POST /v1/admin/adverts/{advertId}/request-changes.
func (h *Handler) RequestAdvertChanges(c *gin.Context, advertID generated.AdvertIdPath) {
	actorID, in, ok := h.adminReasonInput(c)
	if !ok {
		return
	}
	out, err := h.svc.RequestAdvertChanges(c.Request.Context(), actorID, advertID, in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapModerationDetail(out))
}

// RejectAdvert handles POST /v1/admin/adverts/{advertId}/reject.
func (h *Handler) RejectAdvert(c *gin.Context, advertID generated.AdvertIdPath) {
	actorID, in, ok := h.adminReasonInput(c)
	if !ok {
		return
	}
	out, err := h.svc.RejectAdvert(c.Request.Context(), actorID, advertID, in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapModerationDetail(out))
}

// SuspendAdvert handles POST /v1/admin/adverts/{advertId}/suspend.
func (h *Handler) SuspendAdvert(c *gin.Context, advertID generated.AdvertIdPath) {
	actorID, in, ok := h.adminReasonInput(c)
	if !ok {
		return
	}
	out, err := h.svc.SuspendAdvert(c.Request.Context(), actorID, advertID, in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapModerationDetail(out))
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

func (h *Handler) adminReasonInput(c *gin.Context) (uuid.UUID, appadvert.ModerationReasonInput, bool) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return uuid.Nil, appadvert.ModerationReasonInput{}, false
	}
	var req generated.ModerationReasonRequest
	if !bind.JSONBody(c, &req) {
		return uuid.Nil, appadvert.ModerationReasonInput{}, false
	}
	return actorID, appadvert.ModerationReasonInput{
		ExpectedVersion: req.ExpectedVersion,
		Reason:          req.Reason,
	}, true
}

func mapModerationDetail(v domainadvert.ModerationDetailView) generated.ModerationAdvertDetailResponse {
	owner := mapOwnerAdvertBase(v.OwnerView)
	history := make([]generated.StatusHistoryItem, 0, len(v.StatusHistory))
	for _, h := range v.StatusHistory {
		item := generated.StatusHistoryItem{
			ToStatus:  generated.AdvertStatus(h.ToStatus),
			IsSystem:  h.IsSystem,
			CreatedAt: h.CreatedAt,
			Reason:    h.Reason,
		}
		if h.FromStatus != nil {
			fs := generated.AdvertStatus(*h.FromStatus)
			item.FromStatus = &fs
		}
		if h.ActorUserID != nil {
			id := *h.ActorUserID
			item.ActorUserId = &id
		}
		history = append(history, item)
	}
	return generated.ModerationAdvertDetailResponse{
		Id:                     owner.Id,
		Status:                 owner.Status,
		Version:                owner.Version,
		MediaVersion:           owner.MediaVersion,
		CategoryId:             owner.CategoryId,
		DistrictId:             owner.DistrictId,
		HorseId:                owner.HorseId,
		Title:                  owner.Title,
		Description:            owner.Description,
		Price:                  owner.Price,
		Properties:             owner.Properties,
		Media:                  owner.Media,
		PublishedAt:            owner.PublishedAt,
		DeletedAt:              owner.DeletedAt,
		CategoryClearedWarning: owner.CategoryClearedWarning,
		OwnerUserId:            v.OwnerUserID,
		StatusHistory:          history,
	}
}
