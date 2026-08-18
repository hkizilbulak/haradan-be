package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// ListAdvertComments implements GET /v1/adverts/:advertId/comments.
func (s *Server) ListAdvertComments(c *gin.Context, advertId uuid.UUID, params generated.ListAdvertCommentsParams) {
	if s.comment == nil {
		respondNotImplemented(c)
		return
	}
	s.comment.ListAdvertComments(c, advertId, params)
}

// CreateAdvertComment implements POST /v1/adverts/:advertId/comments.
func (s *Server) CreateAdvertComment(c *gin.Context, advertId uuid.UUID) {
	if s.comment == nil {
		respondNotImplemented(c)
		return
	}
	s.comment.CreateAdvertComment(c, advertId)
}
