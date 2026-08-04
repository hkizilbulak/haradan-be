package router_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	apphorse "github.com/hkizilbulak/haradan-be/internal/application/horse"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainhorse "github.com/hkizilbulak/haradan-be/internal/domain/horse"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

type horseRepoStub struct {
	byID       map[uuid.UUID]domainhorse.Horse
	byTJK      map[string]domainhorse.Horse
	prefixHits []domainhorse.Horse
	findIDErr  error
	searchErr  error
}

func (h *horseRepoStub) FindByID(_ context.Context, id uuid.UUID) (domainhorse.Horse, error) {
	if h.findIDErr != nil {
		return domainhorse.Horse{}, h.findIDErr
	}
	horse, ok := h.byID[id]
	if !ok {
		return domainhorse.Horse{}, apperr.NotFound("At bulunamadı.")
	}
	return horse, nil
}

func (h *horseRepoStub) FindByTJKNumber(_ context.Context, tjk string) (domainhorse.Horse, error) {
	horse, ok := h.byTJK[tjk]
	if !ok {
		return domainhorse.Horse{}, apperr.NotFound("At bulunamadı.")
	}
	return horse, nil
}

func (h *horseRepoStub) SearchByNormalizedPrefix(context.Context, string, int) ([]domainhorse.Horse, error) {
	if h.searchErr != nil {
		return nil, h.searchErr
	}
	return append([]domainhorse.Horse(nil), h.prefixHits...), nil
}

func newHorseEngine(repo *horseRepoStub) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := apphorse.NewService(repo)
	srv := handler.NewServer(log, fakeDeps{}, nil, nil, svc, nil, nil, nil, nil, nil, nil, nil, nil)
	return router.New(srv, log)
}

func TestSearchHorsesForSelectionHTTP(t *testing.T) {
	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	year := 2019
	sire := "Sire"
	engine := newHorseEngine(&horseRepoStub{
		prefixHits: []domainhorse.Horse{{
			ID: id, OriginalName: "Ada", TJKNumber: "T1", NameNormalized: "ada",
			BirthYear: &year, SireName: &sire,
		}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/horses?q=ada", nil)
	req.Header.Set("X-Request-ID", "horse-search-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type=%q", ct)
	}
	var body generated.HorseSelectionListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].OriginalName != "Ada" || body.Items[0].TjkNumber != "T1" {
		t.Fatalf("%+v", body)
	}
	raw := rec.Body.String()
	for _, leak := range []string{"nameNormalized", "lastSyncedAt", "password", "refresh"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("leak %s in %s", leak, raw)
		}
	}
}

func TestSearchHorsesEmptyListHTTP(t *testing.T) {
	engine := newHorseEngine(&horseRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/horses", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	raw := rec.Body.String()
	if strings.Contains(raw, `"items":null`) || !strings.Contains(raw, `"items":[]`) {
		t.Fatalf("empty list must be [], got %s", raw)
	}
}

func TestSearchHorsesByTJKNumberHTTP(t *testing.T) {
	id := uuid.New()
	engine := newHorseEngine(&horseRepoStub{
		byTJK: map[string]domainhorse.Horse{
			"9988": {ID: id, OriginalName: "Bolt", TJKNumber: "9988", NameNormalized: "bolt"},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/horses?tjkNumber=9988", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.HorseSelectionListResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Items) != 1 || body.Items[0].TjkNumber != "9988" {
		t.Fatalf("%+v", body)
	}
}

func TestSearchHorsesLimitValidationHTTP(t *testing.T) {
	engine := newHorseEngine(&horseRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/horses?q=a&limit=101", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Code != generated.DomainErrorCodeVALIDATIONERROR || errBody.TraceId == "" {
		t.Fatalf("%+v", errBody)
	}
}

func TestGetHorsePublicDetailHTTP(t *testing.T) {
	id := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	breed := "İngiliz"
	engine := newHorseEngine(&horseRepoStub{
		byID: map[uuid.UUID]domainhorse.Horse{
			id: {
				ID: id, OriginalName: "Ada", TJKNumber: "T9", NameNormalized: "ada",
				Breed: &breed, Detail: []byte(`{"stats":{"wins":1}}`),
			},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/horses/"+id.String(), nil)
	req.Header.Set("X-Request-ID", "horse-detail-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.HorsePublicDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.OriginalName != "Ada" || body.Breed == nil || *body.Breed != "İngiliz" {
		t.Fatalf("%+v", body)
	}
	if body.Detail == nil || body.Detail["stats"] == nil {
		t.Fatalf("detail=%v", body.Detail)
	}
	raw := rec.Body.String()
	for _, leak := range []string{"lastSyncedAt", "lastSeenAt", "sourceUpdatedAt", "nameNormalized"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("leak %s", leak)
		}
	}
}

func TestGetHorsePublicDetailNotFoundHTTP(t *testing.T) {
	engine := newHorseEngine(&horseRepoStub{byID: map[uuid.UUID]domainhorse.Horse{}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/horses/"+uuid.New().String(), nil)
	req.Header.Set("X-Request-ID", "horse-404")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Code != generated.DomainErrorCodeNOTFOUND || errBody.TraceId == "" {
		t.Fatalf("%+v", errBody)
	}
}

func TestGetHorseMalformedIDHTTP(t *testing.T) {
	engine := newHorseEngine(&horseRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/horses/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Code != generated.DomainErrorCodeVALIDATIONERROR {
		t.Fatalf("%+v", errBody)
	}
}

func TestGetHorseInternalErrorHTTP(t *testing.T) {
	engine := newHorseEngine(&horseRepoStub{findIDErr: errors.New("db down")})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/horses/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if strings.Contains(raw, "db down") {
		t.Fatalf("raw error leaked: %s", raw)
	}
}

func TestHorsePublicNoAuthRequiredHTTP(t *testing.T) {
	engine := newHorseEngine(&horseRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/horses", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("public horse search must not require auth")
	}
}

func TestHorseOpsNoLonger501WhenWired(t *testing.T) {
	engine := newHorseEngine(&horseRepoStub{})
	for _, path := range []string{"/api/v1/horses", "/api/v1/horses/" + uuid.New().String()} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotImplemented {
			t.Fatalf("%s still 501", path)
		}
	}
}

func TestFoundationHorseStill501(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := router.NewFoundation(log, fakeDeps{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/horses?q=a", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("foundation horse status=%d", rec.Code)
	}
}
