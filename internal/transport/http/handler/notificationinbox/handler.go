// Package notificationinbox adapts the current-user notification inbox.
package notificationinbox

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

type Handler struct {
	svc     *appnotification.UserNotificationService
	logger  *slog.Logger
	respond ErrorResponder
}

func NewHandler(svc *appnotification.UserNotificationService, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

func (h *Handler) ListMyNotifications(c *gin.Context, params generated.ListMyNotificationsParams) {
	userID, ok := h.principal(c)
	if !ok {
		return
	}
	limit := 0
	if params.Limit != nil {
		limit = int(*params.Limit)
	}
	out, err := h.svc.ListUserNotifications(c.Request.Context(), appnotification.ListUserNotificationsInput{
		UserID: userID, Cursor: params.Cursor, Limit: limit,
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	items := make([]generated.MyNotificationView, 0, len(out.Items))
	for _, item := range out.Items {
		payload := map[string]interface{}{}
		if err := json.Unmarshal(item.Notification.Payload, &payload); err != nil || payload == nil {
			payload = map[string]interface{}{}
		}
		items = append(items, generated.MyNotificationView{
			Id:        item.Notification.ID,
			EventType: generated.NotificationEventType(item.Notification.EventType),
			Title:     item.Notification.Title,
			Body:      item.Notification.Body,
			Payload:   payload,
			CreatedAt: item.Notification.CreatedAt,
			ReadAt:    item.State.ReadAt,
		})
	}
	c.JSON(http.StatusOK, generated.MyNotificationPage{Items: items, HasMore: out.HasMore, NextCursor: out.NextCursor})
}

func (h *Handler) GetMyNotificationUnreadCount(c *gin.Context) {
	userID, ok := h.principal(c)
	if !ok {
		return
	}
	count, err := h.svc.CountUnread(c.Request.Context(), userID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.NotificationUnreadCount{UnreadCount: count})
}

func (h *Handler) MarkMyNotificationRead(c *gin.Context, notificationID generated.NotificationIdPath) {
	userID, ok := h.principal(c)
	if !ok {
		return
	}
	if err := h.svc.MarkRead(c.Request.Context(), userID, uuid.UUID(notificationID)); err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) MarkAllMyNotificationsRead(c *gin.Context) {
	userID, ok := h.principal(c)
	if !ok {
		return
	}
	updated, err := h.svc.MarkAllRead(c.Request.Context(), userID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.MarkAllNotificationsReadResponse{UpdatedCount: int(updated)})
}

func (h *Handler) principal(c *gin.Context) (uuid.UUID, bool) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return uuid.Nil, false
	}
	return p.UserID, true
}
