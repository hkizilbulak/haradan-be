package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware"
)

func respondError(c *gin.Context, logger *slog.Logger, err error) {
	traceID := middleware.RequestIDFromContext(c.Request.Context())
	ae, ok := apperr.As(err)
	if !ok {
		logInternal(logger, err, traceID, "untyped handler error")
		c.JSON(http.StatusInternalServerError, generated.ErrorResponse{
			Code:    generated.DomainErrorCodeINTERNALERROR,
			Message: "Beklenmeyen bir hata oluştu.",
			TraceId: traceID,
		})
		return
	}

	switch ae.Kind {
	case apperr.KindValidation:
		resp := generated.ErrorResponse{
			Code:    generated.DomainErrorCodeVALIDATIONERROR,
			Message: ae.Message,
			TraceId: traceID,
		}
		if len(ae.FieldErrors) > 0 {
			fields := make([]generated.FieldValidationError, 0, len(ae.FieldErrors))
			for _, fe := range ae.FieldErrors {
				fields = append(fields, generated.FieldValidationError{
					Field:   fe.Field,
					Message: fe.Message,
				})
			}
			resp.FieldErrors = &fields
		}
		c.JSON(http.StatusUnprocessableEntity, resp)
	case apperr.KindNotFound:
		c.JSON(http.StatusNotFound, generated.ErrorResponse{
			Code:    generated.DomainErrorCodeNOTFOUND,
			Message: ae.Message,
			TraceId: traceID,
		})
	case apperr.KindConflict:
		// Geo/Catalog 409 paths only declare INVALID_STATE in OpenAPI x-error-codes.
		code := generated.DomainErrorCode(ae.Code)
		if code != generated.DomainErrorCodeINVALIDSTATE {
			code = generated.DomainErrorCodeINVALIDSTATE
		}
		c.JSON(http.StatusConflict, generated.ErrorResponse{
			Code:    code,
			Message: ae.Message,
			TraceId: traceID,
		})
	default:
		logInternal(logger, ae, traceID, "internal application error")
		c.JSON(http.StatusInternalServerError, generated.ErrorResponse{
			Code:    generated.DomainErrorCodeINTERNALERROR,
			Message: "Beklenmeyen bir hata oluştu.",
			TraceId: traceID,
		})
	}
}

func logInternal(logger *slog.Logger, err error, traceID, msg string) {
	if logger == nil {
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		logger.Info(msg, "request_id", traceID, "reason", "context_done")
		return
	}
	logger.Error(msg, "request_id", traceID)
}
