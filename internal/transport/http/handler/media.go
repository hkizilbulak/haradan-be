package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// InitiateMediaUpload implements MEDIA-01.
func (s *Server) InitiateMediaUpload(c *gin.Context) {
	if s.media == nil {
		respondNotImplemented(c)
		return
	}
	s.media.InitiateMediaUpload(c)
}

// ConfirmMediaUpload implements MEDIA-02.
func (s *Server) ConfirmMediaUpload(c *gin.Context, assetId generated.AssetIdPath) {
	if s.media == nil {
		respondNotImplemented(c)
		return
	}
	s.media.ConfirmMediaUpload(c, assetId)
}

// GetMediaProcessingStatus implements MEDIA-03.
func (s *Server) GetMediaProcessingStatus(c *gin.Context, assetId generated.AssetIdPath) {
	if s.media == nil {
		respondNotImplemented(c)
		return
	}
	s.media.GetMediaProcessingStatus(c, assetId)
}

// AttachMediaToAdvert implements MEDIA-04.
func (s *Server) AttachMediaToAdvert(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.media == nil {
		respondNotImplemented(c)
		return
	}
	s.media.AttachMediaToAdvert(c, advertId)
}

// DetachMediaFromAdvert implements MEDIA-05.
func (s *Server) DetachMediaFromAdvert(c *gin.Context, advertId generated.AdvertIdPath, assetId generated.AssetIdPath, params generated.DetachMediaFromAdvertParams) {
	if s.media == nil {
		respondNotImplemented(c)
		return
	}
	s.media.DetachMediaFromAdvert(c, advertId, assetId, params)
}

// ReorderAdvertMedia implements MEDIA-06.
func (s *Server) ReorderAdvertMedia(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.media == nil {
		respondNotImplemented(c)
		return
	}
	s.media.ReorderAdvertMedia(c, advertId)
}

// SetAdvertCover implements MEDIA-07.
func (s *Server) SetAdvertCover(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.media == nil {
		respondNotImplemented(c)
		return
	}
	s.media.SetAdvertCover(c, advertId)
}
