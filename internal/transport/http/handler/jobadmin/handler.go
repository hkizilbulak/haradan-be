// Package jobadmin exposes BO scheduled job definition OpenAPI operations.
package jobadmin

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	appjobadmin "github.com/hkizilbulak/haradan-be/internal/application/jobadmin"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainjobdef "github.com/hkizilbulak/haradan-be/internal/domain/jobdef"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes job admin OpenAPI operations.
type Handler struct {
	svc     *appjobadmin.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs a job admin HTTP handler.
func NewHandler(svc *appjobadmin.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// ListAdminJobs handles GET /v1/admin/jobs.
func (h *Handler) ListAdminJobs(c *gin.Context) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	items, err := h.svc.ListJobs(c.Request.Context(), actorID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out := make([]generated.JobAdminView, 0, len(items))
	for _, item := range items {
		out = append(out, mapJobView(item))
	}
	c.JSON(http.StatusOK, generated.JobAdminListResponse{Items: out})
}

// GetAdminJob handles GET /v1/admin/jobs/{jobId}.
func (h *Handler) GetAdminJob(c *gin.Context, jobID generated.JobIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	out, err := h.svc.GetJob(c.Request.Context(), actorID, jobID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapJobView(out))
}

// UpdateAdminJob handles PATCH /v1/admin/jobs/{jobId}.
func (h *Handler) UpdateAdminJob(c *gin.Context, jobID generated.JobIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	var req generated.UpdateJobRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, err := h.svc.UpdateJob(c.Request.Context(), appjobadmin.UpdateJobInput{
		ActorUserID:     actorID,
		JobID:           jobID,
		ExpectedVersion: req.ExpectedVersion,
		CronExpression:  req.CronExpression,
		IsActive:        req.IsActive,
		TimeoutSeconds:  req.TimeoutSeconds,
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapJobView(out))
}

// RunAdminJob handles POST /v1/admin/jobs/{jobId}/run.
func (h *Handler) RunAdminJob(c *gin.Context, jobID generated.JobIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	var refDate *string
	if c.Request.ContentLength != 0 {
		var req generated.RunJobRequest
		if !bind.JSONBody(c, &req) {
			return
		}
		if req.ReferenceDate != nil {
			s := req.ReferenceDate.Time.Format("2006-01-02")
			refDate = &s
		}
	}
	out, err := h.svc.RunJob(c.Request.Context(), appjobadmin.RunJobInput{
		ActorUserID:   actorID,
		JobID:         jobID,
		ReferenceDate: refDate,
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusAccepted, generated.JobRunAcceptedResponse{
		JobId: jobID,
		RunId: out.BackgroundJobID,
	})
}

// ListAdminJobHistory handles GET /v1/admin/jobs/{jobId}/history.
func (h *Handler) ListAdminJobHistory(
	c *gin.Context,
	jobID generated.JobIdPath,
	params generated.ListAdminJobHistoryParams,
) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	out, err := h.svc.ListHistory(c.Request.Context(), actorID, jobID, params.Cursor, params.Limit)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	items := make([]generated.JobHistoryItem, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, mapHistoryItem(jobID, item))
	}
	c.JSON(http.StatusOK, generated.JobHistoryPage{
		Items:      items,
		NextCursor: out.NextCursor,
		HasMore:    out.HasMore,
	})
}

func (h *Handler) requireAdminBO(c *gin.Context) (uuid.UUID, bool) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return uuid.Nil, false
	}
	if err := authz.RequireAdminBO(p); err != nil {
		h.respond(c, h.logger, err)
		return uuid.Nil, false
	}
	return p.UserID, true
}

func mapJobView(v domainjobdef.JobDefinition) generated.JobAdminView {
	return generated.JobAdminView{
		Id:                    v.ID,
		Key:                   v.JobKey,
		Name:                  v.Name,
		Description:           v.Description,
		JobType:               generated.JobType(v.JobType),
		CronExpression:        v.CronExpression,
		IsActive:              v.IsActive,
		TimeoutSeconds:        v.TimeoutSeconds,
		SupportsReferenceDate: v.SupportsReferenceDate,
		Version:               v.Version,
		CreatedAt:             v.CreatedAt,
		UpdatedAt:             v.UpdatedAt,
	}
}

func mapHistoryItem(jobID uuid.UUID, v domainjobdef.JobExecution) generated.JobHistoryItem {
	execType := generated.JobHistoryItemExecutionTypeSCHEDULED
	if v.ExecutionType != nil {
		execType = generated.JobHistoryItemExecutionType(*v.ExecutionType)
	}
	out := generated.JobHistoryItem{
		Id:            v.ID,
		JobId:         jobID,
		Status:        generated.JobRunStatus(v.Status),
		ExecutionType: execType,
		StartedAt:     v.StartedAt,
		CompletedAt:   v.CompletedAt,
		LastError:     v.LastError,
		CreatedAt:     v.CreatedAt,
		UpdatedAt:     v.UpdatedAt,
	}
	if v.StartedAt != nil && v.CompletedAt != nil {
		ms := int(v.CompletedAt.Sub(*v.StartedAt).Milliseconds())
		if ms < 0 {
			ms = 0
		}
		out.DurationMs = &ms
	}
	if v.ReferenceDate != nil {
		d := openapi_types.Date{Time: v.ReferenceDate.UTC().Truncate(24 * time.Hour)}
		out.ReferenceDate = &d
	}
	return out
}
