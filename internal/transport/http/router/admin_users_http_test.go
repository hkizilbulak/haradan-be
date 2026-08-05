package router_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	appadminuser "github.com/hkizilbulak/haradan-be/internal/application/adminuser"
	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

type adminUserHTTPRepo struct{ users []domainuser.User }

func (r *adminUserHTTPRepo) BeginTx(context.Context) (pgx.Tx, error) { panic("not used") }
func (r *adminUserHTTPRepo) WithTx(pgx.Tx) appadminuser.Repository   { return r }
func (r *adminUserHTTPRepo) ListUsers(_ context.Context, _ *domainuser.Status, _ *domainuser.Role, _ string, _ *time.Time, _ *uuid.UUID, limit int) ([]domainuser.User, error) {
	return r.users[:minAdminHTTP(limit, len(r.users))], nil
}
func (r *adminUserHTTPRepo) FindUser(_ context.Context, userID uuid.UUID) (domainuser.User, error) {
	return domainuser.User{}, apperr.NotFound("user not found")
}
func (r *adminUserHTTPRepo) FindUserForUpdate(context.Context, uuid.UUID) (domainuser.User, error) {
	panic("not used")
}
func (r *adminUserHTTPRepo) GetDetail(context.Context, uuid.UUID, time.Time) (appadminuser.Detail, error) {
	panic("not used")
}
func (r *adminUserHTTPRepo) ActiveSessionCount(context.Context, uuid.UUID, time.Time) (int, error) {
	panic("not used")
}
func (r *adminUserHTTPRepo) UpdateRole(context.Context, uuid.UUID, domainuser.Role, uuid.UUID, time.Time) (domainuser.User, error) {
	panic("not used")
}
func (r *adminUserHTTPRepo) UpdateStatus(context.Context, uuid.UUID, domainuser.Status, uuid.UUID, time.Time) (domainuser.User, error) {
	panic("not used")
}
func (r *adminUserHTTPRepo) RevokeAllSessions(context.Context, uuid.UUID, time.Time, string) error {
	panic("not used")
}
func (r *adminUserHTTPRepo) InsertSecurityEvent(context.Context, domainauth.SecurityEvent) error {
	panic("not used")
}
func (r *adminUserHTTPRepo) ListSecurityEvents(context.Context, uuid.UUID, *domainauth.SecurityEventType, *time.Time, *uuid.UUID, int) ([]domainauth.SecurityEvent, error) {
	panic("not used")
}

func TestAdminUsersListHTTPRequiresAdminBO(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, store, _ := appauth.NewMemoryServiceForTest(t)
	repo := &adminUserHTTPRepo{users: []domainuser.User{{ID: uuid.New(), Email: "target@example.com", FirstName: "Target", LastName: "User", Role: domainuser.RoleUser, Status: domainuser.StatusActive, CreatedAt: time.Now().UTC()}}}
	adminSvc, err := appadminuser.NewService(appadminuser.Config{Repository: repo})
	if err != nil {
		t.Fatal(err)
	}
	engine := router.New(handler.NewServer(log, fakeDeps{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, authSvc).WithAdminUserService(adminSvc), log, router.Options{AuthService: authSvc})
	request := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		if token != "" {
			req.Header.Set("Authorization", token)
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	if rec := request(""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated=%d", rec.Code)
	}
	token := adminUserHTTPLogin(t, authSvc, store)
	rec := request(token)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin list=%d %s", rec.Code, rec.Body.String())
	}
	var out generated.AdminUserListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0].Email != "target@example.com" {
		t.Fatalf("unexpected response: %#v", out)
	}
}

func adminUserHTTPLogin(t *testing.T, authSvc *appauth.Service, store *appauth.MemoryStore) string {
	t.Helper()
	register := func(path, body string) *httptest.ResponseRecorder {
		server := handler.NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeDeps{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, authSvc)
		engine := router.New(server, slog.New(slog.NewTextHandler(io.Discard, nil)))
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	rec := register("/api/v1/auth/register", `{"email":"admin-users@example.com","password":"Password1","firstName":"Admin","lastName":"User"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register=%d %s", rec.Code, rec.Body.String())
	}
	rec = register("/api/v1/auth/login", `{"email":"admin-users@example.com","password":"Password1","clientContext":"PUBLIC_WEB"}`)
	var before generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &before)
	p, err := authSvc.AuthenticateAccessToken(context.Background(), before.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	store.SetUserRole(p.UserID, domainuser.RoleAdmin)
	rec = register("/api/v1/auth/login", `{"email":"admin-users@example.com","password":"Password1","clientContext":"ADMIN_BO"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login=%d %s", rec.Code, rec.Body.String())
	}
	var token generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &token)
	return "Bearer " + token.AccessToken
}

func minAdminHTTP(a, b int) int {
	if a < b {
		return a
	}
	return b
}
