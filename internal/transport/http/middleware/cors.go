package middleware

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

const corsAllowHeaders = "Accept,Authorization,Content-Type,X-Request-ID,Cache-Control,Pragma,If-Match,If-None-Match"
const corsAllowMethods = "GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS"

// CORS allows browser clients from an explicit deployment-configured origin
// list. Requests without Origin (including native mobile clients) are not
// changed. Wildcards and credentialed cross-origin requests are intentionally
// unsupported because the public API uses bearer tokens.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return CORSWithLoopback(allowedOrigins, false)
}

// CORSWithLoopback is CORS plus optional local loopback origins.
// allowLoopback must be true only in non-production environments so Expo/Vite
// can bind an arbitrary localhost port without a 403 preflight.
func CORSWithLoopback(allowedOrigins []string, allowLoopback bool) gin.HandlerFunc {
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
		_, listed := allowed[origin]
		if !listed && !(allowLoopback && isHTTPLoopbackOrigin(origin)) {
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
		headers.Set("Access-Control-Allow-Methods", corsAllowMethods)
		headers.Set("Access-Control-Allow-Headers", corsAllowHeaders)
		headers.Set("Access-Control-Expose-Headers", "X-Request-ID")
		headers.Set("Access-Control-Max-Age", "600")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func isHTTPLoopbackOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Path != "" && u.Path != "/" {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
