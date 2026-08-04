package bind

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware"
)

// JSONBody decodes a required JSON request body into dst.
// Malformed JSON maps to 400 VALIDATION_ERROR without leaking parser details.
func JSONBody(c *gin.Context, dst any) bool {
	traceID := middleware.RequestIDFromContext(c.Request.Context())
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		c.JSON(http.StatusBadRequest, generated.ErrorResponse{
			Code:    generated.DomainErrorCodeVALIDATIONERROR,
			Message: "İstek gövdesi geçersiz.",
			TraceId: traceID,
		})
		return false
	}
	// Reject trailing junk.
	if err := dec.Decode(&struct{}{}); err != io.EOF && err != nil {
		c.JSON(http.StatusBadRequest, generated.ErrorResponse{
			Code:    generated.DomainErrorCodeVALIDATIONERROR,
			Message: "İstek gövdesi geçersiz.",
			TraceId: traceID,
		})
		return false
	}
	return true
}

// BearerToken extracts the Authorization Bearer token.
func BearerToken(c *gin.Context) (string, error) {
	h := strings.TrimSpace(c.GetHeader("Authorization"))
	if h == "" {
		return "", apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
	}
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
	}
	tok := strings.TrimSpace(h[len(prefix):])
	if tok == "" {
		return "", apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
	}
	return tok, nil
}

// IsBodyError reports whether err is a JSON body decode failure (unused helper for tests).
func IsBodyError(err error) bool {
	var se *json.SyntaxError
	var ue *json.UnmarshalTypeError
	return errors.As(err, &se) || errors.As(err, &ue) || errors.Is(err, io.EOF)
}
