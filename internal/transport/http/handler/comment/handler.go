// Package comment adapts comment application use cases to HTTP.
package comment

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appcomment "github.com/hkizilbulak/haradan-be/internal/application/comment"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincomment "github.com/hkizilbulak/haradan-be/internal/domain/comment"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

// ErrorResponder maps domain errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler serves advert comment HTTP operations.
type Handler struct {
	svc     *appcomment.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs a comment HTTP handler.
func NewHandler(svc *appcomment.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// ListAdvertComments handles GET /v1/adverts/{advertId}/comments.
func (h *Handler) ListAdvertComments(c *gin.Context, advertID uuid.UUID, params generated.ListAdvertCommentsParams) {
	limit := 20
	if params.Limit != nil && *params.Limit > 0 {
		limit = *params.Limit
	}
	offset := 0
	if params.Offset != nil && *params.Offset >= 0 {
		offset = *params.Offset
	}

	res, err := h.svc.ListComments(c.Request.Context(), advertID, limit, offset)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}

	items := make([]generated.AdvertCommentItem, 0, len(res.Items))
	for _, item := range res.Items {
		items = append(items, generated.AdvertCommentItem{
			Id:         item.Comment.ID,
			AdvertId:   item.Comment.AdvertID,
			UserId:     item.Comment.UserID,
			AuthorName: item.AuthorName,
			Content:    item.Comment.Content,
			CreatedAt:  item.Comment.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, generated.AdvertCommentListResponse{
		Items:      items,
		TotalCount: res.TotalCount,
	})
}

// CreateAdvertComment handles POST /v1/adverts/{advertId}/comments.
func (h *Handler) CreateAdvertComment(c *gin.Context, advertID uuid.UUID) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok || p.UserID == uuid.Nil {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "giriş yapmanız gerekmektedir"))
		return
	}

	var req generated.CreateAdvertCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respond(c, h.logger, apperr.Validation("geçersiz istek gövdesi", apperr.FieldError{
			Field:   "content",
			Message: "yorum metni veya puan belirtilmelidir",
		}))
		return
	}

	row, err := h.svc.CreateComment(c.Request.Context(), appcomment.CreateCommentInput{
		UserID:   p.UserID,
		AdvertID: advertID,
		Content:  req.Content,
	})
	if err != nil {
		if err == domaincomment.ErrEmptyContent || err == domaincomment.ErrContentTooLong {
			h.respond(c, h.logger, apperr.Validation(err.Error(), apperr.FieldError{
				Field:   "content",
				Message: err.Error(),
			}))
			return
		}

		if err == domaincomment.ErrInvalidRating {
			h.respond(c, h.logger, apperr.Validation("puan 1 ile 5 arasında olmalıdır", apperr.FieldError{
				Field:   "rating",
				Message: err.Error(),
			}))
			return
		}
		if err == domaincomment.ErrAdvertNotCommentable {
			h.respond(c, h.logger, apperr.Conflict("bu ilana yorum yapılamaz"))
			return
		}
		h.respond(c, h.logger, err)
		return
	}

	c.JSON(http.StatusCreated, generated.AdvertCommentItem{
		Id:         row.Comment.ID,
		AdvertId:   row.Comment.AdvertID,
		UserId:     row.Comment.UserID,
		AuthorName: row.AuthorName,
		Content:    row.Comment.Content,
		CreatedAt:  row.Comment.CreatedAt,
	})
}

// DeleteAdvertComment handles DELETE /v1/adverts/{advertId}/comments/{commentId}.
func (h *Handler) DeleteAdvertComment(c *gin.Context, advertID uuid.UUID, commentID uuid.UUID) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok || p.UserID == uuid.Nil {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "giriş yapmanız gerekmektedir"))
		return
	}

	err := h.svc.DeleteComment(c.Request.Context(), advertID, commentID, p.UserID)
	if err != nil {
		if errors.Is(err, domaincomment.ErrUnauthorizedCommentAction) {
			h.respond(c, h.logger, apperr.Forbidden(apperr.CodeForbidden, "yalnızca kendi yaptığınız yorumları silebilirsiniz"))
			return
		}
		if errors.Is(err, domaincomment.ErrCommentNotFound) {
			h.respond(c, h.logger, apperr.NotFound("yorum bulunamadı"))
			return
		}
		h.respond(c, h.logger, err)
		return
	}

	c.Status(http.StatusNoContent)
	c.Writer.WriteHeaderNow()
}
