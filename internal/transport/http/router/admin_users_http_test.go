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
	"github.com/jackc/pgx/v5/pgconn"

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

func (r *adminUserHTTPRepo) BeginTx(context.Context) (pgx.Tx, error) {
	return adminHTTPStubTx{}, nil
}
func (r *adminUserHTTPRepo) WithTx(pgx.Tx) appadminuser.Repository { return r }
func (r *adminUserHTTPRepo) ListUsers(_ context.Context, _ *domainuser.Status, _ *domainuser.Role, _ string, _ *time.Time, _ *uuid.UUID, limit int) ([]domainuser.User, error) {
	return r.users[:minAdminHTTP(limit, len(r.users))], nil
}
func (r *adminUserHTTPRepo) FindUser(_ context.Context, userID uuid.UUID) (domainuser.User, error) {
	for _, u := range r.users {
		if u.ID == userID {
			return u, nil
		}
	}
	return domainuser.User{}, apperr.NotFound("user not found")
}
func (r *adminUserHTTPRepo) FindUserForUpdate(ctx context.Context, userID uuid.UUID) (domainuser.User, error) {
	return r.FindUser(ctx, userID)
}
func (r *adminUserHTTPRepo) GetDetail(ctx context.Context, userID uuid.UUID, _ time.Time) (appadminuser.Detail, error) {
	u, err := r.FindUser(ctx, userID)
	if err != nil {
		return appadminuser.Detail{}, err
	}
	return appadminuser.Detail{User: u}, nil
}
func (r *adminUserHTTPRepo) ActiveSessionCount(context.Context, uuid.UUID, time.Time) (int, error) {
	return 0, nil
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
	return nil
}
func (r *adminUserHTTPRepo) ListSecurityEvents(context.Context, uuid.UUID, *domainauth.SecurityEventType, *time.Time, *uuid.UUID, int) ([]domainauth.SecurityEvent, error) {
	panic("not used")
}
func (r *adminUserHTTPRepo) CreateUser(_ context.Context, user domainuser.User) error {
	for _, existing := range r.users {
		if existing.EmailNormalized == user.EmailNormalized {
			return apperr.Conflict("Bu e-posta adresi zaten kayıtlı.")
		}
	}
	r.users = append(r.users, user)
	return nil
}
func (r *adminUserHTTPRepo) UpdateProfile(_ context.Context, userID uuid.UUID, firstName, lastName string, phone *string, now time.Time) (domainuser.User, error) {
	for i, u := range r.users {
		if u.ID == userID {
			u.FirstName, u.LastName, u.Phone, u.UpdatedAt = firstName, lastName, phone, now
			r.users[i] = u
			return u, nil
		}
	}
	return domainuser.User{}, apperr.NotFound("user not found")
}
func (r *adminUserHTTPRepo) FindUserByNormalizedEmail(_ context.Context, normalized string) (domainuser.User, error) {
	for _, u := range r.users {
		if u.EmailNormalized == normalized {
			return u, nil
		}
	}
	return domainuser.User{}, apperr.NotFound("user not found")
}
func (r *adminUserHTTPRepo) CountActiveAdmins(context.Context) (int, error) { return 0, nil }
func (r *adminUserHTTPRepo) LockActiveAdminGuard(context.Context) error     { return nil }
func (r *adminUserHTTPRepo) InvalidateActiveOneTimeCredentials(context.Context, uuid.UUID, domainauth.OneTimePurpose, time.Time) error {
	return nil
}
func (r *adminUserHTTPRepo) CreateOneTimeCredential(context.Context, domainauth.OneTimeCredential) error {
	return nil
}

type adminHTTPStubTx struct{}

func (adminHTTPStubTx) Begin(context.Context) (pgx.Tx, error) { panic("unused") }
func (adminHTTPStubTx) Commit(context.Context) error          { return nil }
func (adminHTTPStubTx) Rollback(context.Context) error        { return nil }
func (adminHTTPStubTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("unused")
}
func (adminHTTPStubTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("unused") }
func (adminHTTPStubTx) LargeObjects() pgx.LargeObjects                         { panic("unused") }
func (adminHTTPStubTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("unused")
}
func (adminHTTPStubTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	panic("unused")
}
func (adminHTTPStubTx) Query(context.Context, string, ...any) (pgx.Rows, error) { panic("unused") }
func (adminHTTPStubTx) QueryRow(context.Context, string, ...any) pgx.Row        { panic("unused") }
func (adminHTTPStubTx) Conn() *pgx.Conn                                         { panic("unused") }

type recordingAdminEmail struct{ calls int }

func (e *recordingAdminEmail) SendPasswordReset(context.Context, string, string, string) error {
	e.calls++
	return nil
}
func (e *recordingAdminEmail) SendRegistrationVerification(context.Context, string, string, string) error {
	e.calls++
	return nil
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

type adminHTTPHasher struct{}

func (adminHTTPHasher) Hash(password string) (string, error) { return "hash:" + password, nil }

func TestCreateAdminUserHTTP(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, store, _ := appauth.NewMemoryServiceForTest(t)
	repo := &adminUserHTTPRepo{}
	email := &recordingAdminEmail{}
	adminSvc, err := appadminuser.NewService(appadminuser.Config{
		Repository:      repo,
		Hasher:          adminHTTPHasher{},
		EmailSender:     email,
		EmailConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := router.New(handler.NewServer(log, fakeDeps{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, authSvc).WithAdminUserService(adminSvc), log, router.Options{AuthService: authSvc})
	do := func(token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", token)
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	if rec := do("", `{"email":"x@y.com","firstName":"A","lastName":"B","role":"user"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth=%d", rec.Code)
	}
	token := adminUserHTTPLogin(t, authSvc, store)
	if rec := do(token, `{"email":"not-an-email","firstName":"A","lastName":"B","role":"user"}`); rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad body=%d %s", rec.Code, rec.Body.String())
	}
	rec := do(token, `{"email":"invitee@example.com","firstName":"New","lastName":"User","role":"user"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", rec.Code, rec.Body.String())
	}
	var created generated.AdminUserCreateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !created.InvitationEmailSent || created.Email != "invitee@example.com" {
		t.Fatalf("unexpected create response: %#v", created)
	}
	if email.calls != 1 {
		t.Fatalf("email calls=%d", email.calls)
	}
	dup := do(token, `{"email":"invitee@example.com","firstName":"New","lastName":"User","role":"user"}`)
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate=%d %s", dup.Code, dup.Body.String())
	}
}

func TestUpdateAdminUserPhonePatchSemanticsHTTP(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, store, _ := appauth.NewMemoryServiceForTest(t)
	userID := uuid.New()
	phone := "+905321234567"
	updatedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	repo := &adminUserHTTPRepo{users: []domainuser.User{{
		ID: userID, Email: "patch@example.com", EmailNormalized: "patch@example.com",
		FirstName: "Patch", LastName: "User", Phone: &phone,
		Role: domainuser.RoleUser, Status: domainuser.StatusActive,
		CreatedAt: updatedAt.Add(-time.Hour), UpdatedAt: updatedAt,
	}}}
	adminSvc, err := appadminuser.NewService(appadminuser.Config{Repository: repo})
	if err != nil {
		t.Fatal(err)
	}
	engine := router.New(handler.NewServer(log, fakeDeps{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, authSvc).WithAdminUserService(adminSvc), log, router.Options{AuthService: authSvc})
	token := adminUserHTTPLogin(t, authSvc, store)
	patch := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+userID.String(), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", token)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	body := func(phoneJSON *string) string {
		payload := map[string]any{
			"expectedUpdatedAt": repo.users[0].UpdatedAt.Format(time.RFC3339Nano),
			"firstName":         "Patch",
			"lastName":          "User",
		}
		if phoneJSON != nil {
			var value any
			if *phoneJSON == "null" {
				value = nil
			} else {
				value = *phoneJSON
			}
			payload["phone"] = value
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	if rec := patch(body(nil)); rec.Code != http.StatusOK {
		t.Fatalf("omitted phone status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.users[0].Phone == nil || *repo.users[0].Phone != phone {
		t.Fatalf("omitted phone changed value: %#v", repo.users[0].Phone)
	}
	nullValue := "null"
	if rec := patch(body(&nullValue)); rec.Code != http.StatusOK {
		t.Fatalf("clear phone status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.users[0].Phone != nil {
		t.Fatalf("explicit null did not clear phone: %#v", repo.users[0].Phone)
	}
	valid := "0532 123 45 67"
	if rec := patch(body(&valid)); rec.Code != http.StatusOK {
		t.Fatalf("valid phone status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.users[0].Phone == nil || *repo.users[0].Phone != "+905321234567" {
		t.Fatalf("valid phone not normalized: %#v", repo.users[0].Phone)
	}
	before := repo.users[0].UpdatedAt
	invalid := "not-a-phone"
	if rec := patch(body(&invalid)); rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid phone status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.users[0].UpdatedAt.Equal(before) {
		t.Fatal("invalid phone mutated user")
	}
}

func TestResendAdminUserInvitationHTTP(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, store, _ := appauth.NewMemoryServiceForTest(t)
	userID := uuid.New()
	repo := &adminUserHTTPRepo{users: []domainuser.User{{
		ID: userID, Email: "invitee@example.com", EmailNormalized: "invitee@example.com",
		FirstName: "New", LastName: "User", Role: domainuser.RoleUser, Status: domainuser.StatusActive,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}}}
	email := &recordingAdminEmail{}
	adminSvc, err := appadminuser.NewService(appadminuser.Config{
		Repository: repo, Hasher: adminHTTPHasher{}, EmailSender: email, EmailConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := router.New(handler.NewServer(log, fakeDeps{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, authSvc).WithAdminUserService(adminSvc), log, router.Options{AuthService: authSvc})
	token := adminUserHTTPLogin(t, authSvc, store)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+userID.String()+"/invitation/resend", nil)
	req.Header.Set("Authorization", token)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("resend=%d %s", rec.Code, rec.Body.String())
	}
	var out generated.AdminUserCreateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.InvitationEmailSent || out.Email != "invitee@example.com" {
		t.Fatalf("unexpected resend response: %#v", out)
	}
	if email.calls != 1 {
		t.Fatalf("email calls=%d", email.calls)
	}

	unconfigured, err := appadminuser.NewService(appadminuser.Config{
		Repository: repo, Hasher: adminHTTPHasher{}, EmailConfigured: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	engine2 := router.New(handler.NewServer(log, fakeDeps{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, authSvc).WithAdminUserService(unconfigured), log, router.Options{AuthService: authSvc})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+userID.String()+"/invitation/resend", nil)
	req2.Header.Set("Authorization", token)
	rec2 := httptest.NewRecorder()
	engine2.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured resend=%d %s", rec2.Code, rec2.Body.String())
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
