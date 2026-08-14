package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	transportmiddleware "github.com/hkizilbulak/haradan-be/internal/transport/http/middleware"
)

func corsEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(transportmiddleware.CORS([]string{"https://app.example.com"}))
	r.GET("/resource", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestCORSAllowedOriginAndPreflight(t *testing.T) {
	r := corsEngine()

	req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("allow origin=%q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("allow credentials must stay unset, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodHead) {
		t.Fatalf("allow methods=%q does not include HEAD", got)
	}
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-ID" {
		t.Fatalf("expose headers=%q", got)
	}
}

func TestCORSDisallowedOrigin(t *testing.T) {
	r := corsEngine()

	req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow origin=%q", got)
	}
}

func TestCORSLoopbackPreflightInDevelopment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(transportmiddleware.CORSWithLoopback([]string{"https://app.example.com"}, true))
	r.POST("/v1/auth/password/forgot", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, origin := range []string{
		"http://localhost:8083",
		"http://127.0.0.1:8081",
		"http://[::1]:19006",
	} {
		req := httptest.NewRequest(http.MethodOptions, "/v1/auth/password/forgot", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		req.Header.Set("Access-Control-Request-Headers", "content-type")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("origin %s status=%d", origin, rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Fatalf("origin %s allow=%q", origin, got)
		}
		if rec.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatalf("credentials must stay unset")
		}
	}
}

func TestCORSLoopbackRejectedWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(transportmiddleware.CORSWithLoopback(nil, false))
	r.POST("/v1/auth/password/forgot", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodOptions, "/v1/auth/password/forgot", nil)
	req.Header.Set("Origin", "http://localhost:8083")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestCORSLoopbackRejectsSpoofedHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(transportmiddleware.CORSWithLoopback(nil, true))

	req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	req.Header.Set("Origin", "http://localhost.evil.com")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestCORSNativeRequestWithoutOriginPasses(t *testing.T) {
	r := corsEngine()
	req := httptest.NewRequest(http.MethodGet, "/resource", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow origin=%q", got)
	}
}
