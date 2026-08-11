package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS allows browser clients from an explicit deployment-configured origin
// list. Requests without Origin (including native mobile clients) are not
// changed. Wildcards and credentialed cross-origin requests are intentionally
// unsupported because the public API uses bearer tokens.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		if _, ok := allowed[origin]; !ok {
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}
		headers := c.Writer.Header()
		headers.Add("Vary", "Origin")
		headers.Set("Access-Control-Allow-Origin", origin)
		headers.Set("Access-Control-Allow-Methods", "GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS")
		headers.Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID")
		headers.Set("Access-Control-Expose-Headers", "X-Request-ID")
		headers.Set("Access-Control-Max-Age", "600")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
