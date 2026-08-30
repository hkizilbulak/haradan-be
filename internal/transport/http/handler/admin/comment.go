package admin

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appcomment "github.com/hkizilbulak/haradan-be/internal/application/comment"
	domaincomment "github.com/hkizilbulak/haradan-be/internal/domain/comment"
)

type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

type CommentHandler struct {
	svc     *appcomment.Service
	logger  *slog.Logger
	respond ErrorResponder
}

func NewCommentHandler(svc *appcomment.Service, logger *slog.Logger, respond ErrorResponder) *CommentHandler {
	return &CommentHandler{svc: svc, logger: logger, respond: respond}
}

func (h *CommentHandler) RegisterRoutes(r gin.IRouter) {
	adminComments := r.Group("/v1/admin/comments")
	{
		adminComments.GET("", h.List)
		adminComments.PATCH("/:id/approve", h.Approve)
		adminComments.PATCH("/:id/reject", h.Reject)
		adminComments.DELETE("/:id", h.Delete)
	}
}

func (h *CommentHandler) List(c *gin.Context) {
	statusStr := c.Query("status")
	limitStr := c.Query("limit")
	offsetStr := c.Query("offset")

	var status *domaincomment.Status
	if statusStr != "" {
		st := domaincomment.Status(statusStr)
		status = &st
	}

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	res, err := h.svc.AdminListComments(c.Request.Context(), status, limit, offset)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *CommentHandler) Approve(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	err = h.svc.ApproveComment(c.Request.Context(), id)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *CommentHandler) Reject(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	err = h.svc.RejectComment(c.Request.Context(), id)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *CommentHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	err = h.svc.AdminDeleteComment(c.Request.Context(), id)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.Status(http.StatusNoContent)
}
