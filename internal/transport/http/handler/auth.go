package handler

import (
	"github.com/gin-gonic/gin"
)

// RegisterUser handles AUTH-01.
func (s *Server) RegisterUser(c *gin.Context) {
	if s.auth == nil {
		respondNotImplemented(c)
		return
	}
	s.auth.RegisterUser(c)
}

// Login handles AUTH-04.
func (s *Server) Login(c *gin.Context) {
	if s.auth == nil {
		respondNotImplemented(c)
		return
	}
	s.auth.Login(c)
}

// RefreshSession handles AUTH-05.
func (s *Server) RefreshSession(c *gin.Context) {
	if s.auth == nil {
		respondNotImplemented(c)
		return
	}
	s.auth.RefreshSession(c)
}

// LogoutCurrentSession handles AUTH-06.
func (s *Server) LogoutCurrentSession(c *gin.Context) {
	if s.auth == nil {
		respondNotImplemented(c)
		return
	}
	s.auth.LogoutCurrentSession(c)
}

// VerifyRegistrationEmail handles AUTH-02.
func (s *Server) VerifyRegistrationEmail(c *gin.Context) {
	if s.auth == nil {
		respondNotImplemented(c)
		return
	}
	s.auth.VerifyRegistrationEmail(c)
}

// ResendRegistrationEmailVerification handles AUTH-03.
func (s *Server) ResendRegistrationEmailVerification(c *gin.Context) {
	if s.auth == nil {
		respondNotImplemented(c)
		return
	}
	s.auth.ResendRegistrationEmailVerification(c)
}
