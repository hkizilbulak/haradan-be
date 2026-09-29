package handler

import (
	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ListCommunicationTemplates implements the generated interface.
func (s *Server) ListCommunicationTemplates(c *gin.Context) {
	if s.communicationTemplate == nil {
		respondNotImplemented(c)
		return
	}
	s.communicationTemplate.ListCommunicationTemplates(c)
}

// CreateCommunicationTemplate implements the generated interface.
func (s *Server) CreateCommunicationTemplate(c *gin.Context) {
	if s.communicationTemplate == nil {
		respondNotImplemented(c)
		return
	}
	s.communicationTemplate.CreateCommunicationTemplate(c)
}

// UpdateCommunicationTemplate implements the generated interface.
func (s *Server) UpdateCommunicationTemplate(c *gin.Context, id openapi_types.UUID) {
	if s.communicationTemplate == nil {
		respondNotImplemented(c)
		return
	}
	s.communicationTemplate.UpdateCommunicationTemplate(c, id)
}

// DeleteCommunicationTemplate implements the generated interface.
func (s *Server) DeleteCommunicationTemplate(c *gin.Context, id openapi_types.UUID) {
	if s.communicationTemplate == nil {
		respondNotImplemented(c)
		return
	}
	s.communicationTemplate.DeleteCommunicationTemplate(c, id)
}
