package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

type fakeDeps struct {
	err error
}

func (f fakeDeps) Ping(context.Context) error { return f.err }

func TestGetHealthHealthy(t *testing.T) {
	engine := router.NewFoundation(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeDeps{})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("X-Request-ID", "health-ok-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type=%q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Request-ID") != "health-ok-1" {
		t.Fatalf("request id=%q", rec.Header().Get("X-Request-ID"))
	}
	var body generated.HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != generated.Ok {
		t.Fatalf("status=%q", body.Status)
	}
}

func TestGetHealthDependencyUnavailable(t *testing.T) {
	engine := router.NewFoundation(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		fakeDeps{err: errors.New("connection refused secret=super-db-password")},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("X-Request-ID", "health-fail-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Code != generated.DomainErrorCodeDEPENDENCYUNAVAILABLE {
		t.Fatalf("code=%q", body.Code)
	}
	if body.Message == "" || body.TraceId != "health-fail-1" {
		t.Fatalf("body=%+v", body)
	}
	if strings.Contains(rec.Body.String(), "super-db-password") || strings.Contains(rec.Body.String(), "connection refused") {
		t.Fatalf("leaked dependency error text: %s", rec.Body.String())
	}
}

func TestRequestIDPreservedAndRejected(t *testing.T) {
	engine := router.NewFoundation(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeDeps{})

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
	engine := router.NewFoundation(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeDeps{})

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
	if body.Code != generated.DomainErrorCodeINTERNALERROR || body.TraceId != "login-trace-1" {
		t.Fatalf("body=%+v", body)
	}
}

func TestOpenAPIRouteCount(t *testing.T) {
	engine := router.NewFoundation(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeDeps{})
	if got := router.CountOpenAPIRoutes(engine); got != 133 {
		t.Fatalf("route count=%d, want 133", got)
	}
}

func TestWrongBasePathsNotFound(t *testing.T) {
	engine := router.NewFoundation(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeDeps{})
	for _, path := range []string{"/health", "/v1/auth/login"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d", path, rec.Code)
		}
	}
}

func TestDeleteCommentRouteMatch(t *testing.T) {
	engine := router.NewFoundation(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeDeps{})
	advID := uuid.New()
	cmtID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/adverts/%s/comments/%s", advID, cmtID), nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 Not Implemented for Foundation router, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}
