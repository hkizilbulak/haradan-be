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

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

type packagingTestEnv struct {
	authSvc   *appauth.Service
	authStore *appauth.MemoryStore
	pkgStore  *apppackaging.MemoryStore
	do        func(method, path, body, auth string) *httptest.ResponseRecorder
}

func newPackagingEngine(t *testing.T) *packagingTestEnv {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, authStore, _ := appauth.NewMemoryServiceForTest(t)

	pkgStore := apppackaging.NewMemoryStore()
	now := time.Now().UTC()
	dur := 30
	pkgStore.PutPackage(domainpackaging.Package{
		ID:   uuid.MustParse("a0000000-0000-4000-8000-000000000001"),
		Code: domainpackaging.PackageCode("STARTER"), DisplayName: "Starter",
		CurrencyCode: "TRY", IsActive: true, SortOrder: 10, Version: 1,
		BenefitsJSON: []byte(`["a"]`), CreatedAt: now, UpdatedAt: now,
	})
	pkgStore.PutPackage(domainpackaging.Package{
		ID:   uuid.MustParse("a0000000-0000-4000-8000-000000000003"),
		Code: domainpackaging.PackageCode("ADVANCED"), DisplayName: "Advanced",
		CurrencyCode: "TRY", DefaultDurationDays: &dur, AllowsUrgent: true,
		ShowcaseEligible: true, SearchPriority: 100, IsActive: true, SortOrder: 30,
		Version: 1, BenefitsJSON: []byte(`[]`), CreatedAt: now, UpdatedAt: now,
	})

	pkgSvc, err := apppackaging.NewMemoryService(pkgStore, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := handler.NewServer(
		log, fakeDeps{}, nil, nil, nil, nil, nil, nil,
		pkgSvc, nil, nil, nil, authSvc,
	)
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
		req.Header.Set("X-Request-ID", "packaging-http-1")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	return &packagingTestEnv{
		authSvc: authSvc, authStore: authStore, pkgStore: pkgStore, do: do,
	}
}

func (env *packagingTestEnv) registerLogin(t *testing.T, email, clientContext string) (string, uuid.UUID) {
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
	env.pkgStore.PutUser(domainuser.User{
		ID: principal.UserID, Status: domainuser.StatusActive, EmailVerifiedAt: &now,
		Role: domainuser.RoleUser,
	})
	return "Bearer " + tokens.AccessToken, principal.UserID
}

func (env *packagingTestEnv) registerAdminBO(t *testing.T, email string) (string, uuid.UUID) {
	t.Helper()
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
	env.pkgStore.PutUser(domainuser.User{
		ID: principal.UserID, Status: domainuser.StatusActive, EmailVerifiedAt: &now,
		Role: domainuser.RoleAdmin,
	})
	return "Bearer " + tokens.AccessToken, principal.UserID
}

func TestPackagingAdminAuthzHTTP(t *testing.T) {
	env := newPackagingEngine(t)
	adminAuth, _ := env.registerAdminBO(t, "admin-pkg@example.com")
	userAuth, _ := env.registerLogin(t, "user-pkg@example.com", "PUBLIC_WEB")

	rec := env.do(http.MethodGet, "/api/v1/admin/packages", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token=%d", rec.Code)
	}

	rec = env.do(http.MethodGet, "/api/v1/admin/packages", "", userAuth)
	assertError(t, rec, http.StatusForbidden, generated.DomainErrorCodeFORBIDDEN)

	rec = env.do(http.MethodGet, "/api/v1/admin/packages", "", adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin=%d %s", rec.Code, rec.Body.String())
	}
	var list generated.PackageAdminListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("items=%d", len(list.Items))
	}
}

func TestPackagingAssignGetCancelUrgentHTTP(t *testing.T) {
	env := newPackagingEngine(t)
	adminAuth, _ := env.registerAdminBO(t, "admin-flow@example.com")
	ownerAuth, ownerID := env.registerLogin(t, "owner-flow@example.com", "PUBLIC_WEB")

	now := time.Now().UTC()
	advertID := uuid.New()
	env.pkgStore.PutAdvert(domainadvert.Advert{
		ID: advertID, OwnerUserID: ownerID, Status: domainadvert.StatusPublished,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	})

	rec := env.do(http.MethodPut, "/api/v1/admin/adverts/"+advertID.String()+"/package",
		`{"packageCode":"ADVANCED"}`, adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("assign=%d %s", rec.Code, rec.Body.String())
	}
	var assigned generated.AdvertPackageAssignmentView
	if err := json.Unmarshal(rec.Body.Bytes(), &assigned); err != nil {
		t.Fatal(err)
	}
	if assigned.PackageCode != "ADVANCED" || assigned.Status != generated.PackageAssignmentStatusACTIVE {
		t.Fatalf("%+v", assigned)
	}

	rec = env.do(http.MethodGet, "/api/v1/admin/adverts/"+advertID.String()+"/package", "", adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("get=%d %s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodPut, "/api/v1/adverts/"+advertID.String()+"/urgent", "", ownerAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("urgent=%d %s", rec.Code, rec.Body.String())
	}
	var urgent generated.AdvertUrgentActivationView
	if err := json.Unmarshal(rec.Body.Bytes(), &urgent); err != nil {
		t.Fatal(err)
	}
	if urgent.FeatureCode != generated.URGENT || urgent.Status != generated.AdvertUrgentActivationViewStatusACTIVE {
		t.Fatalf("%+v", urgent)
	}

	rec = env.do(http.MethodDelete, "/api/v1/adverts/"+advertID.String()+"/urgent", "", ownerAuth)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("deactivate=%d %s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodPost, "/api/v1/admin/adverts/"+advertID.String()+"/package/cancel",
		`{"reason":"test"}`, adminAuth)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("cancel=%d %s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodGet, "/api/v1/admin/adverts/"+advertID.String()+"/package", "", adminAuth)
	assertError(t, rec, http.StatusNotFound, generated.DomainErrorCodeNOTFOUND)
}

func TestUpdateAdminPackageNullableHTTP(t *testing.T) {
	env := newPackagingEngine(t)
	adminAuth, _ := env.registerAdminBO(t, "pkg-null@example.com")
	// set values
	rec := env.do(http.MethodPatch, "/api/v1/admin/packages/STARTER",
		`{"expectedVersion":1,"description":"d1","badgeText":"b1","displayPrice":{"amountMinor":500,"currency":"TRY"},"defaultDurationDays":7}`,
		adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("set=%d %s", rec.Code, rec.Body.String())
	}
	var view generated.PackageAdminView
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	// omit keeps
	rec = env.do(http.MethodPatch, "/api/v1/admin/packages/STARTER",
		`{"expectedVersion":`+itoa(view.Version)+`}`,
		adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("omit=%d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Description == nil || *view.Description != "d1" || view.BadgeText == nil || *view.BadgeText != "b1" ||
		view.DisplayPrice == nil || view.DisplayPrice.AmountMinor != 500 ||
		view.DefaultDurationDays == nil || *view.DefaultDurationDays != 7 {
		t.Fatalf("omit lost values: %+v", view)
	}
	// null clears
	rec = env.do(http.MethodPatch, "/api/v1/admin/packages/STARTER",
		`{"expectedVersion":`+itoa(view.Version)+`,"description":null,"badgeText":null,"displayPrice":null,"defaultDurationDays":null}`,
		adminAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear=%d %s", rec.Code, rec.Body.String())
	}
	var cleared generated.PackageAdminView
	if err := json.Unmarshal(rec.Body.Bytes(), &cleared); err != nil {
		t.Fatal(err)
	}
	if cleared.Description != nil || cleared.BadgeText != nil || cleared.DisplayPrice != nil || cleared.DefaultDurationDays != nil {
		t.Fatalf("null did not clear: %+v body=%s", cleared, rec.Body.String())
	}
}

func itoa(v int) string {
	return strings.TrimSpace(strings.ReplaceAll(jsonNumber(v), " ", ""))
}

func jsonNumber(v int) string {
	b, _ := json.Marshal(v)
	return string(b)
}
