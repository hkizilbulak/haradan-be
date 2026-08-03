package router_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

func TestGetHealth(t *testing.T) {
	engine := router.NewFoundation(slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type=%q", ct)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID")
	}

	var body generated.HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != generated.Ok {
		t.Fatalf("status=%q, want ok", body.Status)
	}
}

func TestRequestIDPreservedAndRejected(t *testing.T) {
	engine := router.NewFoundation(slog.New(slog.NewTextHandler(io.Discard, nil)))

	t.Run("safe inbound id preserved", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		req.Header.Set("X-Request-ID", "req-abc_123:ok")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if got := rec.Header().Get("X-Request-ID"); got != "req-abc_123:ok" {
			t.Fatalf("X-Request-ID=%q", got)
		}
	})

	t.Run("unsafe inbound id replaced", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		req.Header.Set("X-Request-ID", "bad id\nwith-control")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		got := rec.Header().Get("X-Request-ID")
		if got == "" || got == "bad id\nwith-control" || strings.ContainsAny(got, "\n\r") {
			t.Fatalf("X-Request-ID=%q was not replaced", got)
		}
	})
}

func TestLoginNotImplemented(t *testing.T) {
	engine := router.NewFoundation(slog.New(slog.NewTextHandler(io.Discard, nil)))

	payload := []byte(`{"email":"user@example.com","password":"Password1!","clientContext":"PUBLIC_WEB"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "login-trace-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var body generated.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Code != generated.DomainErrorCodeINTERNALERROR {
		t.Fatalf("code=%q", body.Code)
	}
	if body.Message == "" {
		t.Fatal("empty message")
	}
	if body.TraceId != "login-trace-1" {
		t.Fatalf("traceId=%q", body.TraceId)
	}
}

func TestOpenAPIRouteCount(t *testing.T) {
	engine := router.NewFoundation(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if got := router.CountOpenAPIRoutes(engine); got != 88 {
		t.Fatalf("route count=%d, want 88", got)
	}
}

func TestWrongBasePathsNotFound(t *testing.T) {
	engine := router.NewFoundation(slog.New(slog.NewTextHandler(io.Discard, nil)))

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/health"},
		{http.MethodPost, "/v1/auth/login"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s status=%d, want 404", tc.method, tc.path, rec.Code)
		}
	}
}
