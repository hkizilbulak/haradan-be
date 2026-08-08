package router_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	apptjk "github.com/hkizilbulak/haradan-be/internal/application/tjk"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

type tjkHTTPRepo struct {
	runs   map[uuid.UUID]domain.Run
	active map[string]uuid.UUID
}

func newTJKHTTPRepo() *tjkHTTPRepo {
	return &tjkHTTPRepo{runs: map[uuid.UUID]domain.Run{}, active: map[string]uuid.UUID{}}
}

func (f *tjkHTTPRepo) key(source, scope string) string { return source + "|" + scope }

func (f *tjkHTTPRepo) CreateRun(_ context.Context, r domain.Run) error {
	k := f.key(r.SourceAdapter, r.Scope)
	if id, ok := f.active[k]; ok {
		if run, exists := f.runs[id]; exists && (run.Status == domain.RunQueued || run.Status == domain.RunRunning) {
			return apperr.Conflict("Bu kaynak ve kapsam için zaten aktif bir TJK senkronizasyonu var.")
		}
	}
	f.runs[r.ID] = r
	f.active[k] = r.ID
	return nil
}
func (f *tjkHTTPRepo) EnqueueRun(context.Context, uuid.UUID, time.Time) error { return nil }
func (f *tjkHTTPRepo) ListRuns(context.Context, *string, *string, int) ([]domain.Run, bool, error) {
	return nil, false, nil
}
func (f *tjkHTTPRepo) GetRun(_ context.Context, id uuid.UUID) (domain.Run, error) {
	r, ok := f.runs[id]
	if !ok {
		return domain.Run{}, apperr.NotFound("TJK senkronizasyonu bulunamadı.")
	}
	return r, nil
}
func (f *tjkHTTPRepo) RequestCancel(_ context.Context, id uuid.UUID, version int, now time.Time) (domain.Run, error) {
	r, ok := f.runs[id]
	if !ok || r.Version != version {
		return domain.Run{}, apperr.Conflict("TJK senkronizasyonu güncellenemedi.")
	}
	switch r.Status {
	case domain.RunQueued:
		r.Status = domain.RunCancelled
		r.CancelRequestedAt = &now
		r.CancelledAt = &now
		r.CompletedAt = &now
		r.Version++
		f.runs[id] = r
		delete(f.active, f.key(r.SourceAdapter, r.Scope))
		return r, nil
	case domain.RunRunning:
		r.CancelRequestedAt = &now
		r.Version++
		f.runs[id] = r
		return r, nil
	default:
		return domain.Run{}, apperr.Conflict("TJK senkronizasyonu güncellenemedi.")
	}
}
func (*tjkHTTPRepo) ListItemErrors(context.Context, uuid.UUID, *string, *string, int) ([]domain.ItemError, bool, error) {
	return nil, false, nil
}
func (*tjkHTTPRepo) SetItemErrorStatus(context.Context, uuid.UUID, string, time.Time) (domain.ItemError, error) {
	return domain.ItemError{}, nil
}

func TestTJKTriggerAndQueuedCancelHTTP(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, store, _ := appauth.NewMemoryServiceForTest(t)
	repo := newTJKHTTPRepo()
	tjkSvc, err := apptjk.NewService(apptjk.Config{Repo: repo, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	engine := router.New(
		handler.NewServer(log, fakeDeps{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, authSvc).WithTJKService(tjkSvc),
		log,
		router.Options{AuthService: authSvc},
	)
	token := adminUserHTTPLogin(t, authSvc, store)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		var rdr io.Reader
		if body != "" {
			rdr = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, rdr)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", token)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}

	rec := do(http.MethodPost, "/api/v1/admin/tjk/sync-runs", `{"mode":"FULL","sourceAdapter":"TJK_HTTP","scope":"HORSES"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("trigger=%d %s", rec.Code, rec.Body.String())
	}
	var run generated.TJKSyncRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if run.Status != generated.TJKSyncRunStatusQUEUED {
		t.Fatalf("status=%s", run.Status)
	}

	cancelBody := `{"expectedVersion":` + strconv.Itoa(int(run.Version)) + `}`
	rec = do(http.MethodPost, "/api/v1/admin/tjk/sync-runs/"+run.Id.String()+"/cancel", cancelBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel=%d %s", rec.Code, rec.Body.String())
	}
	var cancelled generated.TJKSyncRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &cancelled); err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != generated.TJKSyncRunStatusCANCELLED {
		t.Fatalf("queued cancel must be CANCELLED, got %s", cancelled.Status)
	}

	rec = do(http.MethodPost, "/api/v1/admin/tjk/sync-runs/"+cancelled.Id.String()+"/cancel", `{"expectedVersion":`+strconv.Itoa(int(cancelled.Version))+`}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("terminal cancel=%d %s", rec.Code, rec.Body.String())
	}

	rec = do(http.MethodPost, "/api/v1/admin/tjk/sync-runs", `{"mode":"FULL","sourceAdapter":"TJK_HTTP","scope":"HORSES"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("retrigger after queued cancel=%d %s", rec.Code, rec.Body.String())
	}
}
