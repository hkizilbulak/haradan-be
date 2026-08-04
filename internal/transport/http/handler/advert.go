package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// ListMyAdverts implements ADVERT-OWNER-02.
func (s *Server) ListMyAdverts(c *gin.Context, params generated.ListMyAdvertsParams) {
	if s.advert == nil {
		respondNotImplemented(c)
		return
	}
	s.advert.ListMyAdverts(c, params)
}

// CreateAdvertDraft implements ADVERT-OWNER-01.
func (s *Server) CreateAdvertDraft(c *gin.Context) {
	if s.advert == nil {
		respondNotImplemented(c)
		return
	}
	s.advert.CreateAdvertDraft(c)
}

// SoftDeleteAdvertDraft implements ADVERT-OWNER-09.
func (s *Server) SoftDeleteAdvertDraft(c *gin.Context, advertId generated.AdvertIdPath, params generated.SoftDeleteAdvertDraftParams) {
	if s.advert == nil {
		respondNotImplemented(c)
		return
	}
	s.advert.SoftDeleteAdvertDraft(c, advertId, params)
}

// GetMyAdvert implements ADVERT-OWNER-03.
func (s *Server) GetMyAdvert(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.advert == nil {
		respondNotImplemented(c)
		return
	}
	s.advert.GetMyAdvert(c, advertId)
}

// UpdateAdvertDraftDetails implements ADVERT-OWNER-04.
func (s *Server) UpdateAdvertDraftDetails(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.advert == nil {
		respondNotImplemented(c)
		return
	}
	s.advert.UpdateAdvertDraftDetails(c, advertId)
}

// ArchiveAdvert implements ADVERT-OWNER-11.
func (s *Server) ArchiveAdvert(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.advert == nil {
		respondNotImplemented(c)
		return
	}
	s.advert.ArchiveAdvert(c, advertId)
}

// ChangeAdvertDraftCategory implements ADVERT-OWNER-05.
func (s *Server) ChangeAdvertDraftCategory(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.advert == nil {
		respondNotImplemented(c)
		return
	}
	s.advert.ChangeAdvertDraftCategory(c, advertId)
}

// ReplaceAdvertDynamicProperties implements ADVERT-OWNER-06.
func (s *Server) ReplaceAdvertDynamicProperties(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.advert == nil {
		respondNotImplemented(c)
		return
	}
	s.advert.ReplaceAdvertDynamicProperties(c, advertId)
}

// ResubmitAdvertForReview implements ADVERT-OWNER-08.
func (s *Server) ResubmitAdvertForReview(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.advert == nil {
		respondNotImplemented(c)
		return
	}
	s.advert.ResubmitAdvertForReview(c, advertId)
}

// MarkAdvertSold implements ADVERT-OWNER-10.
func (s *Server) MarkAdvertSold(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.advert == nil {
		respondNotImplemented(c)
		return
	}
	s.advert.MarkAdvertSold(c, advertId)
}

// SubmitAdvertForReview implements ADVERT-OWNER-07.
func (s *Server) SubmitAdvertForReview(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.advert == nil {
		respondNotImplemented(c)
		return
	}
	s.advert.SubmitAdvertForReview(c, advertId)
}
