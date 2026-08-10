package tjk

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	app "github.com/hkizilbulak/haradan-be/internal/application/tjk"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

type ErrorResponder func(*gin.Context, *slog.Logger, error)
type Handler struct {
	svc     *app.Service
	logger  *slog.Logger
	respond ErrorResponder
}

func NewHandler(s *app.Service, l *slog.Logger, r ErrorResponder) *Handler { return &Handler{s, l, r} }
func (h *Handler) admin(c *gin.Context) (uuid.UUID, bool) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return uuid.Nil, false
	}
	if e := authz.RequireAdminBO(p); e != nil {
		h.respond(c, h.logger, e)
		return uuid.Nil, false
	}
	return p.UserID, true
}
func (h *Handler) Trigger(c *gin.Context) {
	actor, ok := h.admin(c)
	if !ok {
		return
	}
	var req generated.TriggerTJKSyncRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, e := h.svc.Trigger(c.Request.Context(), actor, string(req.Mode), req.SourceAdapter)
	if e != nil {
		h.respond(c, h.logger, e)
		return
	}
	c.JSON(http.StatusAccepted, mapRun(out))
}
func (h *Handler) List(c *gin.Context, p generated.ListTJKSyncRunsParams) {
	if _, ok := h.admin(c); !ok {
		return
	}
	var cursor, status *string
	if p.Cursor != nil {
		x := string(*p.Cursor)
		cursor = &x
	}
	if p.Status != nil {
		x := string(*p.Status)
		status = &x
	}
	limit := 0
	if p.Limit != nil {
		limit = int(*p.Limit)
	}
	items, more, e := h.svc.List(c.Request.Context(), cursor, status, limit)
	if e != nil {
		h.respond(c, h.logger, e)
		return
	}
	out := make([]generated.TJKSyncRunResponse, 0, len(items))
	for _, x := range items {
		out = append(out, mapRun(x))
	}
	var next *string
	if more && len(items) > 0 {
		x := items[len(items)-1].CreatedAt.Format(time.RFC3339Nano)
		next = &x
	}
	c.JSON(http.StatusOK, generated.TJKSyncRunListResponse{Items: out, HasMore: more, NextCursor: next})
}
func (h *Handler) Get(c *gin.Context, id generated.RunIdPath) {
	if _, ok := h.admin(c); !ok {
		return
	}
	out, e := h.svc.Get(c.Request.Context(), uuid.UUID(id))
	if e != nil {
		h.respond(c, h.logger, e)
		return
	}
	c.JSON(http.StatusOK, mapRun(out))
}
func (h *Handler) Cancel(c *gin.Context, id generated.RunIdPath) {
	if _, ok := h.admin(c); !ok {
		return
	}
	var req generated.ExpectedVersionRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, e := h.svc.Cancel(c.Request.Context(), uuid.UUID(id), req.ExpectedVersion)
	if e != nil {
		h.respond(c, h.logger, e)
		return
	}
	c.JSON(http.StatusOK, mapRun(out))
}
func (h *Handler) ListErrors(c *gin.Context, id generated.RunIdPath, p generated.ListTJKSyncItemErrorsParams) {
	if _, ok := h.admin(c); !ok {
		return
	}
	var cursor, status *string
	if p.Cursor != nil {
		x := string(*p.Cursor)
		cursor = &x
	}
	if p.Status != nil {
		x := string(*p.Status)
		status = &x
	}
	limit := 0
	if p.Limit != nil {
		limit = int(*p.Limit)
	}
	items, more, e := h.svc.ListErrors(c.Request.Context(), uuid.UUID(id), cursor, status, limit)
	if e != nil {
		h.respond(c, h.logger, e)
		return
	}
	out := make([]generated.TJKSyncItemErrorResponse, 0, len(items))
	for _, x := range items {
		out = append(out, mapError(x))
	}
	var next *string
	if more && len(items) > 0 {
		x := items[len(items)-1].CreatedAt.Format(time.RFC3339Nano)
		next = &x
	}
	c.JSON(http.StatusOK, generated.TJKSyncItemErrorListResponse{Items: out, HasMore: more, NextCursor: next})
}
func (h *Handler) Resolve(c *gin.Context, id generated.ErrorIdPath) { h.set(c, id, false) }
func (h *Handler) Ignore(c *gin.Context, id generated.ErrorIdPath)  { h.set(c, id, true) }
func (h *Handler) set(c *gin.Context, id generated.ErrorIdPath, ignore bool) {
	if _, ok := h.admin(c); !ok {
		return
	}
	var out domain.ItemError
	var e error
	if ignore {
		out, e = h.svc.IgnoreError(c.Request.Context(), uuid.UUID(id))
	} else {
		out, e = h.svc.ResolveError(c.Request.Context(), uuid.UUID(id))
	}
	if e != nil {
		h.respond(c, h.logger, e)
		return
	}
	c.JSON(http.StatusOK, mapError(out))
}
func mapRun(x domain.Run) generated.TJKSyncRunResponse {
	return generated.TJKSyncRunResponse{Id: openapi_types.UUID(x.ID), Mode: generated.TJKSyncMode(x.Mode), Status: generated.TJKSyncRunStatus(x.Status), SourceAdapter: x.SourceAdapter, Scope: generated.TJKSyncScope(x.Scope), TriggerKind: generated.TJKTriggerKind(x.TriggerKind), CancelRequestedAt: x.CancelRequestedAt, CompletedAt: x.CompletedAt, ConflictCount: x.ConflictCount, CreatedAt: x.CreatedAt, CreatedCount: x.CreatedCount, FailedCount: x.FailedCount, LastErrorSummary: x.LastErrorSummary, SkippedCount: x.SkippedCount, StartedAt: x.StartedAt, TotalCount: x.TotalCount, UnchangedCount: x.UnchangedCount, UpdatedCount: x.UpdatedCount, Version: x.Version}
}
func mapError(x domain.ItemError) generated.TJKSyncItemErrorResponse {
	return generated.TJKSyncItemErrorResponse{Id: openapi_types.UUID(x.ID), RunId: openapi_types.UUID(x.RunID), TjkNumber: x.TJKNumber, HorseId: uuidPtr(x.HorseID), ErrorClass: x.ErrorClass, Status: generated.TJKSyncItemErrorStatus(x.Status), Message: x.Message, CreatedAt: x.CreatedAt, ResolvedAt: x.ResolvedAt}
}
func uuidPtr(v *uuid.UUID) *openapi_types.UUID {
	if v == nil {
		return nil
	}
	x := openapi_types.UUID(*v)
	return &x
}
