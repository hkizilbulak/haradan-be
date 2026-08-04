package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// ListMyFavorites implements FAVORITE-03.
func (s *Server) ListMyFavorites(c *gin.Context, params generated.ListMyFavoritesParams) {
	if s.favorite == nil {
		respondNotImplemented(c)
		return
	}
	s.favorite.ListMyFavorites(c, params)
}

// AddFavorite implements FAVORITE-01.
func (s *Server) AddFavorite(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.favorite == nil {
		respondNotImplemented(c)
		return
	}
	s.favorite.AddFavorite(c, advertId)
}

// RemoveFavorite implements FAVORITE-02.
func (s *Server) RemoveFavorite(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.favorite == nil {
		respondNotImplemented(c)
		return
	}
	s.favorite.RemoveFavorite(c, advertId)
}
