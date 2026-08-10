package notificationtpl

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes notification template admin OpenAPI operations.
type Handler struct {
	svc     *appnotification.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs a notification template HTTP handler.
func NewHandler(svc *appnotification.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// ListAdminNotificationTemplates handles GET /v1/admin/notification-templates.
func (h *Handler) ListAdminNotificationTemplates(c *gin.Context) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	items, err := h.svc.ListTemplates(c.Request.Context(), actorID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out := make([]generated.NotificationTemplateAdminView, 0, len(items))
	for _, t := range items {
		out = append(out, mapTemplateView(t))
	}
	c.JSON(http.StatusOK, generated.NotificationTemplateAdminListResponse{Items: out})
}

// GetAdminNotificationTemplate handles GET /v1/admin/notification-templates/{eventType}.
func (h *Handler) GetAdminNotificationTemplate(c *gin.Context, eventType generated.NotificationEventTypePath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	out, err := h.svc.GetTemplateByEventType(
		c.Request.Context(),
		actorID,
		domainnotification.TemplateEventType(eventType),
	)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapTemplateView(out))
}

// UpdateAdminNotificationTemplate handles PATCH /v1/admin/notification-templates/{eventType}.
func (h *Handler) UpdateAdminNotificationTemplate(c *gin.Context, eventType generated.NotificationEventTypePath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	in, err := decodeUpdateTemplateInput(c, actorID, domainnotification.TemplateEventType(eventType))
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out, err := h.svc.UpdateTemplate(c.Request.Context(), in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapTemplateView(out))
}

func mapTemplateView(t domainnotification.NotificationTemplate) generated.NotificationTemplateAdminView {
	return generated.NotificationTemplateAdminView{
		Id:                   t.ID,
		EventType:            generated.NotificationEventType(t.EventType),
		Name:                 t.Name,
		InAppTitleTemplate:   t.InAppTitleTemplate,
		InAppBodyTemplate:    t.InAppBodyTemplate,
		ResendTemplateId:     t.ResendTemplateID,
		EmailSubjectFallback: t.EmailSubjectFallback,
		IsActive:             t.IsActive,
		Version:              t.Version,
		UpdatedByUserId:      t.UpdatedByUserID,
		CreatedAt:            t.CreatedAt,
		UpdatedAt:            t.UpdatedAt,
	}
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
