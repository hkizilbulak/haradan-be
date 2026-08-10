// Package favorite adapts favorite application use cases to HTTP.
package favorite

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appfavorite "github.com/hkizilbulak/haradan-be/internal/application/favorite"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

// ErrorResponder maps domain errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler serves FAVORITE-01..03.
type Handler struct {
	svc     *appfavorite.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs a favorite HTTP handler.
func NewHandler(svc *appfavorite.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// ListMyFavorites handles GET /v1/me/favorites.
func (h *Handler) ListMyFavorites(c *gin.Context, params generated.ListMyFavoritesParams) {
	userID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	out, err := h.svc.ListMyFavorites(c.Request.Context(), userID, appfavorite.ListInput{
		Cursor: params.Cursor,
		Limit:  params.Limit,
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapListResult(out))
}

// AddFavorite handles PUT /v1/me/favorites/{advertId}.
func (h *Handler) AddFavorite(c *gin.Context, advertID generated.AdvertIdPath) {
	userID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	out, err := h.svc.AddFavorite(c.Request.Context(), userID, advertID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapMutation(out))
}

// RemoveFavorite handles DELETE /v1/me/favorites/{advertId}.
func (h *Handler) RemoveFavorite(c *gin.Context, advertID generated.AdvertIdPath) {
	userID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	out, err := h.svc.RemoveFavorite(c.Request.Context(), userID, advertID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapMutation(out))
}

func (h *Handler) requirePrincipal(c *gin.Context) (uuid.UUID, bool) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return uuid.Nil, false
	}
	return p.UserID, true
}

func mapMutation(v appfavorite.MutationView) generated.FavoriteMutationResponse {
	return generated.FavoriteMutationResponse{
		AdvertId:  v.AdvertID,
		Favorited: v.Favorited,
	}
}

func mapListResult(v appfavorite.ListResult) generated.FavoriteListResponse {
	items := make([]generated.FavoriteListItem, 0, len(v.Items))
	for _, item := range v.Items {
		items = append(items, mapListItem(item))
	}
	return generated.FavoriteListResponse{
		Items:      items,
		HasMore:    v.HasMore,
		NextCursor: v.NextCursor,
	}
}

func mapListItem(item appfavorite.ListItemView) generated.FavoriteListItem {
	out := generated.FavoriteListItem{
		AdvertId:          item.AdvertID,
		Available:         item.Available,
		UnavailableReason: item.UnavailableReason,
	}
	if item.Card != nil {
		card := mapCard(*item.Card)
		out.Card = &card
	}
	return out
}

func mapCard(card appfavorite.CardView) generated.PublishedAdvertCard {
	out := generated.PublishedAdvertCard{
		Id:          card.ID,
		Title:       card.Title,
		PublishedAt: card.PublishedAt,
		CategoryId:  card.CategoryID,
		DistrictId:  card.DistrictID,
		ProvinceId:  card.ProvinceID,
		HorseId:     card.HorseID,
		Cover:       nil,
		IsFavorite:  boolPtr(true),
	}
	if card.Price != nil {
		out.Price = &generated.Money{
			AmountMinor: int(card.Price.AmountMinor),
			Currency:    card.Price.Currency,
		}
	}
	return out
}

func boolPtr(v bool) *bool { return &v }
