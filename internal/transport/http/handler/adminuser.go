package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

func (s *Server) ListUsers(c *gin.Context, params generated.ListUsersParams) {
	if s.adminuser == nil {
		respondNotImplemented(c)
		return
	}
	s.adminuser.ListUsers(c, params)
}

func (s *Server) GetUserAdminDetail(c *gin.Context, userID generated.UserIdPath) {
	if s.adminuser == nil {
		respondNotImplemented(c)
		return
	}
	s.adminuser.GetUserAdminDetail(c, userID)
}

func (s *Server) ChangeUserRole(c *gin.Context, userID generated.UserIdPath) {
	if s.adminuser == nil {
		respondNotImplemented(c)
		return
	}
	s.adminuser.ChangeUserRole(c, userID)
}

func (s *Server) ChangeUserStatus(c *gin.Context, userID generated.UserIdPath) {
	if s.adminuser == nil {
		respondNotImplemented(c)
		return
	}
	s.adminuser.ChangeUserStatus(c, userID)
}

func (s *Server) ListUserSecurityEvents(c *gin.Context, userID generated.UserIdPath, params generated.ListUserSecurityEventsParams) {
	if s.adminuser == nil {
		respondNotImplemented(c)
		return
	}
	s.adminuser.ListUserSecurityEvents(c, userID, params)
}
