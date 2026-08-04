package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware"
)

const parseValidationMessage = "İstek parametreleri geçersiz."

// generatedParseErrorHandler maps oapi-codegen bind/parse failures to ErrorResponse.
// Generated wrappers always pass http.StatusBadRequest for these failures.
func generatedParseErrorHandler(logger *slog.Logger) func(*gin.Context, error, int) {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context, err error, statusCode int) {
		if c.Writer.Written() {
			return
		}
		traceID := middleware.RequestIDFromContext(c.Request.Context())
		if statusCode == 0 {
			statusCode = http.StatusBadRequest
		}
		// Keep generated semantic: parse/bind failures are 400 VALIDATION_ERROR.
		if statusCode != http.StatusBadRequest {
			statusCode = http.StatusBadRequest
		}

		logger.Info("request parameter parse failed",
			"request_id", traceID,
			"method", c.Request.Method,
			"path", c.FullPath(),
		)

		c.AbortWithStatusJSON(statusCode, generated.ErrorResponse{
			Code:    generated.DomainErrorCodeVALIDATIONERROR,
			Message: parseValidationMessage,
			TraceId: traceID,
		})
	}
}
