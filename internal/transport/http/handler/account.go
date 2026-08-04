package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// GetMyProfile handles ACCOUNT-01.
func (s *Server) GetMyProfile(c *gin.Context) {
	if s.account == nil {
		respondNotImplemented(c)
		return
	}
	s.account.GetMyProfile(c)
}

// UpdateMyProfile handles ACCOUNT-02.
func (s *Server) UpdateMyProfile(c *gin.Context) {
	if s.account == nil {
		respondNotImplemented(c)
		return
	}
	s.account.UpdateMyProfile(c)
}

// LogoutAllSessions handles AUTH-07.
func (s *Server) LogoutAllSessions(c *gin.Context) {
	if s.account == nil {
		respondNotImplemented(c)
		return
	}
	s.account.LogoutAllSessions(c)
}

// ListMySessions handles AUTH-08.
func (s *Server) ListMySessions(c *gin.Context, params generated.ListMySessionsParams) {
	if s.account == nil {
		respondNotImplemented(c)
		return
	}
	s.account.ListMySessions(c, params)
}

// RevokeMySession handles AUTH-09.
func (s *Server) RevokeMySession(c *gin.Context, sessionId generated.SessionIdPath) {
	if s.account == nil {
		respondNotImplemented(c)
		return
	}
	s.account.RevokeMySession(c, sessionId)
}
