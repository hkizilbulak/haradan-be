package admin

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	appcomment "github.com/hkizilbulak/haradan-be/internal/application/comment"
	domaincomment "github.com/hkizilbulak/haradan-be/internal/domain/comment"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
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
	if !h.requireAdminOrCallCenterBO(c) {
		return
	}

	limitStr := c.Query("limit")
	offsetStr := c.Query("offset")
	
	// Read filter fields
	advertTitle := c.Query("advertTitle")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	
	// Support both `?statuses=PENDING,PUBLISHED` or `?status=PENDING&status=PUBLISHED`
	// Actually, since we updated the frontend to send an array, it might be sent as `status=PENDING&status=PUBLISHED`
	// Or maybe a comma-separated string `statuses=PENDING,PUBLISHED`. Let's support both just in case.
	var statuses []domaincomment.Status
	if stArray := c.QueryArray("statuses"); len(stArray) > 0 {
		for _, s := range stArray {
			statuses = append(statuses, domaincomment.Status(s))
		}
	} else if stArray := c.QueryArray("status"); len(stArray) > 0 {
		for _, s := range stArray {
			statuses = append(statuses, domaincomment.Status(s))
		}
	} else if stStr := c.Query("statuses"); stStr != "" {
		statuses = append(statuses, domaincomment.Status(stStr))
	} else if stStr := c.Query("status"); stStr != "" {
		statuses = append(statuses, domaincomment.Status(stStr))
	}

	filter := appcomment.AdminCommentFilter{
		Statuses:    statuses,
		AdvertTitle: advertTitle,
		StartDate:   startDate,
		EndDate:     endDate,
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

	res, err := h.svc.AdminListComments(c.Request.Context(), filter, limit, offset)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *CommentHandler) Approve(c *gin.Context) {
	if !h.requireAdminOrCallCenterBO(c) {
		return
	}

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
	if !h.requireAdminOrCallCenterBO(c) {
		return
	}

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
	if !h.requireAdminOrCallCenterBO(c) {
		return
	}

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

func (h *CommentHandler) requireAdminOrCallCenterBO(c *gin.Context) bool {
	principal, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		return false
	}
	if err := authz.RequireAdminOrCallCenterBO(principal); err != nil {
		h.respond(c, h.logger, err)
		return false
	}
	return true
}
