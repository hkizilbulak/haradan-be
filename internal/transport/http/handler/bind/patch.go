package bind

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware"
)

const MalformedBodyMessage = "İstek gövdesi geçersiz."

// PatchObject decodes a JSON object into a raw field map while rejecting
// unknown keys and trailing junk. Callers use presence + IsJSONNull to
// implement omit / null / value tri-state patches.
func PatchObject(c *gin.Context, allowed map[string]struct{}) (map[string]json.RawMessage, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, apperr.BadRequest(apperr.CodeValidation, MalformedBodyMessage)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, apperr.BadRequest(apperr.CodeValidation, MalformedBodyMessage)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF && err != nil {
		return nil, apperr.BadRequest(apperr.CodeValidation, MalformedBodyMessage)
	}
	for key := range raw {
		if _, ok := allowed[key]; !ok {
			return nil, apperr.BadRequest(apperr.CodeValidation, MalformedBodyMessage)
		}
	}
	return raw, nil
}

// RespondBadBody writes the standard 400 VALIDATION_ERROR body envelope.
func RespondBadBody(c *gin.Context) {
	c.JSON(http.StatusBadRequest, generated.ErrorResponse{
		Code:    generated.DomainErrorCodeVALIDATIONERROR,
		Message: MalformedBodyMessage,
		TraceId: middleware.RequestIDFromContext(c.Request.Context()),
	})
}

// IsJSONNull reports whether raw is an explicit JSON null.
func IsJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// RequireExpectedVersion extracts required expectedVersion from a patch object.
func RequireExpectedVersion(raw map[string]json.RawMessage) (int, error) {
	evRaw, ok := raw["expectedVersion"]
	if !ok || IsJSONNull(evRaw) {
		return 0, apperr.BadRequest(apperr.CodeValidation, MalformedBodyMessage)
	}
	var expectedVersion int
	if err := json.Unmarshal(evRaw, &expectedVersion); err != nil {
		return 0, apperr.BadRequest(apperr.CodeValidation, MalformedBodyMessage)
	}
	return expectedVersion, nil
}
