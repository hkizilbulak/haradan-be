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

	appadvert "github.com/hkizilbulak/haradan-be/internal/application/advert"
	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

type moderationTestEnv struct {
	authSvc     *appauth.Service
	authStore   *appauth.MemoryStore
	advertStore *appadvert.MemoryStore
	category    uuid.UUID
	district    uuid.UUID
	do          func(method, path, body, auth string) *httptest.ResponseRecorder
}

func newModerationEngine(t *testing.T) *moderationTestEnv {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, authStore, _ := appauth.NewMemoryServiceForTest(t)

	advertStore := appadvert.NewMemoryStore()
	category := uuid.New()
	district := uuid.New()
	advertStore.PutCategory(
		domaincatalog.Category{ID: category, Slug: "yarim-kan", Name: "Yarım Kan", IsActive: true},
		0,
		[]domaincatalog.Property{
			{ID: uuid.New(), CategoryID: category, Code: "age", DataType: "INTEGER", IsRequired: true},
		},
	)
	advertStore.PutDistrict(domaingeo.District{ID: district, ProvinceID: uuid.New(), Name: "Çankaya", IsActive: true})

	advertSvc, err := appadvert.NewMemoryService(advertStore, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := handler.NewServer(log, fakeDeps{}, nil, nil, nil, advertSvc, nil, nil, nil, nil, nil, nil, authSvc)
	engine := router.New(srv, log, router.Options{AuthService: authSvc})
	do := func(method, path, body, auth string) *httptest.ResponseRecorder {
		var rdr io.Reader
		if body != "" {
			rdr = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, rdr)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("X-Request-ID", "moderation-http-1")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	return &moderationTestEnv{
		authSvc: authSvc, authStore: authStore, advertStore: advertStore,
		category: category, district: district, do: do,
	}
}

func (env *moderationTestEnv) registerLogin(t *testing.T, email, clientContext string) (string, uuid.UUID, *generated.AuthTokenResponse) {
	t.Helper()
	rec := env.do(http.MethodPost, "/api/v1/auth/register",
		`{"email":"`+email+`","password":"Password1","firstName":"Ada","lastName":"Lovelace"}`, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register=%d %s", rec.Code, rec.Body.String())
	}
	rec = env.do(http.MethodPost, "/api/v1/auth/login",
		`{"email":"`+email+`","password":"Password1","clientContext":"`+clientContext+`"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login=%d %s", rec.Code, rec.Body.String())
	}
	var tokens generated.AuthTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &tokens); err != nil {
		t.Fatal(err)
	}
	principal, err := env.authSvc.AuthenticateAccessToken(context.Background(), tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	env.advertStore.PutUser(domainuser.User{
		ID: principal.UserID, Status: domainuser.StatusActive, EmailVerifiedAt: &now,
	})
	return "Bearer " + tokens.AccessToken, principal.UserID, &tokens
}

func (env *moderationTestEnv) registerAdminBO(t *testing.T, email string) (string, uuid.UUID) {
	t.Helper()
	// Register + promote before ADMIN_BO login.
	rec := env.do(http.MethodPost, "/api/v1/auth/register",
		`{"email":"`+email+`","password":"Password1","firstName":"Ada","lastName":"Admin"}`, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register=%d %s", rec.Code, rec.Body.String())
	}
	rec = env.do(http.MethodPost, "/api/v1/auth/login",
		`{"email":"`+email+`","password":"Password1","clientContext":"PUBLIC_WEB"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("pre-login=%d %s", rec.Code, rec.Body.String())
	}
	var pre generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &pre)
	p, err := env.authSvc.AuthenticateAccessToken(context.Background(), pre.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	env.authStore.SetUserRole(p.UserID, domainuser.RoleAdmin)

	rec = env.do(http.MethodPost, "/api/v1/auth/login",
		`{"email":"`+email+`","password":"Password1","clientContext":"ADMIN_BO"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("admin login=%d %s", rec.Code, rec.Body.String())
	}
	var tokens generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &tokens)
	principal, err := env.authSvc.AuthenticateAccessToken(context.Background(), tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	env.advertStore.PutUser(domainuser.User{
		ID: principal.UserID, Status: domainuser.StatusActive, EmailVerifiedAt: &now, Role: domainuser.RoleAdmin,
	})
	return "Bearer " + tokens.AccessToken, principal.UserID
}

func (env *moderationTestEnv) seedPending(ownerID uuid.UUID) domainadvert.Advert {
	title := "Moderasyon ilanı"
	desc := "Açıklama"
	addr := "Ataköy Mah. No:1"
	now := time.Now().UTC()
	a := domainadvert.Advert{
		ID: uuid.New(), OwnerUserID: ownerID,
		CategoryID: &env.category, DistrictID: &env.district,
		Title: &title, Description: &desc, Address: &addr,
		Status:     domainadvert.StatusPendingReview,
		Properties: json.RawMessage(`{"age":5}`),
		Version:    1, MediaVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	env.advertStore.PutAdvert(a)

	// Moderation approval runs validateForSubmission which now requires an
	// attached READY cover. Seed a MASTER_READY cover relation in-memory.
	assetID := uuid.New()
	env.advertStore.PutMediaRelations(a.ID, []domainadvert.MediaRelation{{
		AssetID:         assetID,
		DisplayOrder:    0,
		IsCover:         true,
		LifecycleStatus: string(domainmedia.AssetMasterReady),
	}})
	return a
}

func TestAdminModerationAuthzHTTP(t *testing.T) {
	env := newModerationEngine(t)
	adminAuth, _ := env.registerAdminBO(t, "admin-mod@example.com")
	userAuth, ownerID, userTokens := env.registerLogin(t, "user-mod@example.com", "PUBLIC_WEB")
	pending := env.seedPending(ownerID)

	rec := env.do(http.MethodGet, "/api/v1/admin/adverts/moderation", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token=%d", rec.Code)
	}

	rec = env.do(http.MethodGet, "/api/v1/admin/adverts/moderation", "", "Bearer "+userTokens.RefreshToken)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh bearer=%d", rec.Code)
	}

	rec = env.do(http.MethodGet, "/api/v1/admin/adverts/moderation", "", userAuth)
	assertError(t, rec, http.StatusForbidden, generated.DomainErrorCodeFORBIDDEN)

	rec = env.do(http.MethodGet, "/api/v1/admin/adverts/moderation", "", adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin=%d %s", rec.Code, rec.Body.String())
	}

	// Other BO_AUTH routes require authentication before handler logic.
	rec = env.do(http.MethodGet, "/api/v1/admin/banners", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("banners tokensız=%d", rec.Code)
	}
	rec = env.do(http.MethodGet, "/api/v1/admin/users", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("users tokensız=%d", rec.Code)
	}

	_ = pending
}

func TestAdminModerationInactiveAndRevokedHTTP(t *testing.T) {
	env := newModerationEngine(t)
	adminAuth, adminID := env.registerAdminBO(t, "admin-inactive@example.com")
	p, err := env.authSvc.AuthenticateAccessToken(context.Background(), strings.TrimPrefix(adminAuth, "Bearer "))
	if err != nil {
		t.Fatal(err)
	}

	env.authStore.SetUserStatus(adminID, domainuser.StatusDisabled)
	rec := env.do(http.MethodGet, "/api/v1/admin/adverts/moderation", "", adminAuth)
	assertError(t, rec, http.StatusForbidden, generated.DomainErrorCodeACCOUNTINACTIVE)

	env.authStore.SetUserStatus(adminID, domainuser.StatusActive)
	env.authStore.RevokeSession(p.SessionID, time.Now().UTC(), "TEST")
	rec = env.do(http.MethodGet, "/api/v1/admin/adverts/moderation", "", adminAuth)
	assertError(t, rec, http.StatusUnauthorized, generated.DomainErrorCodeSESSIONREVOKED)
}

func TestAdminModerationHappyPathHTTP(t *testing.T) {
	env := newModerationEngine(t)
	adminAuth, adminID := env.registerAdminBO(t, "admin-happy@example.com")
	_, ownerID, _ := env.registerLogin(t, "owner-happy@example.com", "PUBLIC_WEB")
	pending := env.seedPending(ownerID)

	rec := env.do(http.MethodGet, "/api/v1/admin/adverts/moderation", "", adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("list=%d %s", rec.Code, rec.Body.String())
	}
	var queue generated.ModerationQueueResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &queue)
	if len(queue.Items) != 1 || queue.Items[0].Id != pending.ID {
		t.Fatalf("%+v", queue)
	}
	if queue.Items[0].Media == nil || len(queue.Items[0].Media) != 0 {
		t.Fatalf("media=%#v", queue.Items[0].Media)
	}

	rec = env.do(http.MethodGet, "/api/v1/admin/adverts/"+pending.ID.String(), "", adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail=%d %s", rec.Code, rec.Body.String())
	}
	var detail generated.ModerationAdvertDetailResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &detail)
	if detail.OwnerUserId != ownerID || detail.StatusHistory == nil {
		t.Fatalf("%+v", detail)
	}
	raw := rec.Body.String()
	for _, leak := range []string{"passwordHash", "securityStamp", "objectKey", "failure_reason"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("leak %s in %s", leak, raw)
		}
	}

	rec = env.do(http.MethodPost, "/api/v1/admin/adverts/"+pending.ID.String()+"/approve",
		`{"expectedVersion":1}`, adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve=%d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &detail)
	if detail.Status != generated.PUBLISHED || detail.PublishedAt == nil || detail.Version != 2 {
		t.Fatalf("%+v", detail)
	}
	if len(detail.StatusHistory) != 1 || detail.StatusHistory[0].ActorUserId == nil ||
		*detail.StatusHistory[0].ActorUserId != adminID {
		t.Fatalf("history=%+v", detail.StatusHistory)
	}

	rec = env.do(http.MethodPost, "/api/v1/admin/adverts/"+pending.ID.String()+"/suspend",
		`{"expectedVersion":2,"reason":"Şikayet"}`, adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("suspend=%d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &detail)
	if detail.Status != generated.SUSPENDED {
		t.Fatalf("%+v", detail)
	}
}

func TestAdminModerationActionsHTTP(t *testing.T) {
	env := newModerationEngine(t)
	adminAuth, _ := env.registerAdminBO(t, "admin-actions@example.com")
	_, ownerID, _ := env.registerLogin(t, "owner-actions@example.com", "PUBLIC_WEB")

	changes := env.seedPending(ownerID)
	rec := env.do(http.MethodPost, "/api/v1/admin/adverts/"+changes.ID.String()+"/request-changes",
		`{"expectedVersion":1,"reason":"Düzelt"}`, adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("request-changes=%d %s", rec.Code, rec.Body.String())
	}

	reject := env.seedPending(ownerID)
	rec = env.do(http.MethodPost, "/api/v1/admin/adverts/"+reject.ID.String()+"/reject",
		`{"expectedVersion":1,"reason":"Ret"}`, adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("reject=%d %s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodPost, "/api/v1/admin/adverts/"+reject.ID.String()+"/reject",
		`{"expectedVersion":2,"reason":" "}`, adminAuth)
	assertError(t, rec, http.StatusUnprocessableEntity, generated.DomainErrorCodeVALIDATIONERROR)

	rec = env.do(http.MethodPost, "/api/v1/admin/adverts/"+reject.ID.String()+"/approve",
		`{"expectedVersion":2}`, adminAuth)
	assertError(t, rec, http.StatusConflict, generated.DomainErrorCodeINVALIDSTATE)

	rec = env.do(http.MethodPost, "/api/v1/admin/adverts/"+changes.ID.String()+"/approve",
		`{"expectedVersion":99}`, adminAuth)
	assertError(t, rec, http.StatusConflict, generated.DomainErrorCodeSTALEVERSION)

	rec = env.do(http.MethodGet, "/api/v1/admin/adverts/"+uuid.New().String(), "", adminAuth)
	assertError(t, rec, http.StatusNotFound, generated.DomainErrorCodeNOTFOUND)

	rec = env.do(http.MethodPost, "/api/v1/admin/adverts/not-a-uuid/approve", `{"expectedVersion":1}`, adminAuth)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed uuid=%d", rec.Code)
	}

	rec = env.do(http.MethodPost, "/api/v1/admin/adverts/"+reject.ID.String()+"/approve", `{`, adminAuth)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed body=%d", rec.Code)
	}
}

func TestAdminModerationOpsNoLonger501HTTP(t *testing.T) {
	env := newModerationEngine(t)
	adminAuth, _ := env.registerAdminBO(t, "admin-501@example.com")
	_, ownerID, _ := env.registerLogin(t, "owner-501@example.com", "PUBLIC_WEB")
	pending := env.seedPending(ownerID)

	cases := []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/v1/admin/adverts/moderation", ""},
		{http.MethodGet, "/api/v1/admin/adverts/" + pending.ID.String(), ""},
		{http.MethodPost, "/api/v1/admin/adverts/" + pending.ID.String() + "/approve", `{"expectedVersion":1}`},
	}
	for _, tc := range cases {
		rec := env.do(tc.method, tc.path, tc.body, adminAuth)
		if rec.Code == http.StatusNotImplemented {
			t.Fatalf("%s %s still 501", tc.method, tc.path)
		}
	}

	for _, path := range []string{
		"/api/v1/admin/banners",
		"/api/v1/admin/users",
		"/api/v1/adverts",
	} {
		rec := env.do(http.MethodGet, path, "", adminAuth)
		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("%s status=%d", path, rec.Code)
		}
	}
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code generated.DomainErrorCode) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status=%d body=%s want=%d", rec.Code, rec.Body.String(), status)
	}
	var body generated.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != code || body.TraceId == "" {
		t.Fatalf("%+v", body)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type=%s", rec.Header().Get("Content-Type"))
	}
}
