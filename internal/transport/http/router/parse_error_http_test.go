package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

func TestMalformedPathUUIDReturns400ValidationError(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	cases := []struct {
		name string
		path string
	}{
		{name: "ListDistrictsByProvince", path: "/api/v1/provinces/not-a-uuid/districts"},
		{name: "GetCategoryFormDefinition", path: "/api/v1/categories/bad-id/form"},
		{name: "GetHorsePublicDetail501Surface", path: "/api/v1/horses/not-a-uuid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("X-Request-ID", "parse-path-1")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			assertParseValidationError(t, rec, "parse-path-1")
		})
	}
}

func TestMalformedQueryUUIDSearchDistricts(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/districts/search?provinceId=not-uuid&q=a", nil)
	req.Header.Set("X-Request-ID", "parse-query-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assertParseValidationError(t, rec, "parse-query-1")
}

func TestMalformedLimitQueryReturns400(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces/search?q=ank&limit=abc", nil)
	req.Header.Set("X-Request-ID", "parse-limit-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assertParseValidationError(t, rec, "parse-limit-1")
}

func TestMissingRequiredQueryExpectedVersionReturns400(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me/adverts/11111111-1111-1111-1111-111111111111", nil)
	req.Header.Set("X-Request-ID", "parse-required-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assertParseValidationError(t, rec, "parse-required-1")
}

func TestApplicationValidationStill422(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces/search?q=ank&limit=101", nil)
	req.Header.Set("X-Request-ID", "app-val-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != generated.DomainErrorCodeVALIDATIONERROR || body.TraceId != "app-val-1" {
		t.Fatalf("body=%+v", body)
	}
}

func TestValidUUIDReachesBusinessOr501(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{provinceOK: map[uuid.UUID]bool{}}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces/22222222-2222-2222-2222-222222222222/districts", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/horses/22222222-2222-2222-2222-222222222222", nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("horse status=%d", rec.Code)
	}
}

func assertParseValidationError(t *testing.T, rec *httptest.ResponseRecorder, wantTrace string) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type=%q", rec.Header().Get("Content-Type"))
	}
	raw := rec.Body.String()
	if strings.Contains(raw, "Invalid format") || strings.Contains(raw, `"msg"`) {
		t.Fatalf("leaked parser payload: %s", raw)
	}
	var body generated.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if body.Code != generated.DomainErrorCodeVALIDATIONERROR {
		t.Fatalf("code=%q", body.Code)
	}
	if body.Message == "" || body.TraceId != wantTrace {
		t.Fatalf("body=%+v", body)
	}
	if rec.Header().Get("X-Request-ID") != wantTrace {
		t.Fatalf("response request id=%q", rec.Header().Get("X-Request-ID"))
	}
}
