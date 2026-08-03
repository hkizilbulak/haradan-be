package middleware

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type contextKey string

const (
	requestIDHeader     = "X-Request-ID"
	maxRequestIDLength  = 128
	requestIDContextKey = contextKey("request_id")
)

// RequestID attaches a request ID to the request context and response headers.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := sanitizeRequestID(c.GetHeader(requestIDHeader))
		if id == "" {
			id = uuid.NewString()
		}
		ctx := context.WithValue(c.Request.Context(), requestIDContextKey, id)
		c.Request = c.Request.WithContext(ctx)
		c.Writer.Header().Set(requestIDHeader, id)
		c.Next()
	}
}

// RequestIDFromContext returns the request ID stored in context, or empty string.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(requestIDContextKey).(string); ok {
		return v
	}
	return ""
}

func sanitizeRequestID(raw string) string {
	id := strings.TrimSpace(raw)
	if id == "" {
		return ""
	}
	if utf8.RuneCountInString(id) > maxRequestIDLength {
		return ""
	}
	for _, r := range id {
		if r < 32 || r == 127 || unicode.IsControl(r) {
			return ""
		}
		// Allow common request-id alphabets: alnum, dash, underscore, colon, dot.
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == ':' || r == '.' {
			continue
		}
		return ""
	}
	return id
}
