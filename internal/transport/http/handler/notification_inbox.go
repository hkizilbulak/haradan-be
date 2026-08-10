package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

func (s *Server) ListMyNotifications(c *gin.Context, params generated.ListMyNotificationsParams) {
	if s.inbox == nil {
		respondNotImplemented(c)
		return
	}
	s.inbox.ListMyNotifications(c, params)
}

func (s *Server) GetMyNotificationUnreadCount(c *gin.Context) {
	if s.inbox == nil {
		respondNotImplemented(c)
		return
	}
	s.inbox.GetMyNotificationUnreadCount(c)
}

func (s *Server) MarkMyNotificationRead(c *gin.Context, notificationID generated.NotificationIdPath) {
	if s.inbox == nil {
		respondNotImplemented(c)
		return
	}
	s.inbox.MarkMyNotificationRead(c, notificationID)
}

func (s *Server) MarkAllMyNotificationsRead(c *gin.Context) {
	if s.inbox == nil {
		respondNotImplemented(c)
		return
	}
	s.inbox.MarkAllMyNotificationsRead(c)
}
