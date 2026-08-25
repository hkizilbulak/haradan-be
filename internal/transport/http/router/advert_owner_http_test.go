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
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

// advertTestEnv wires a real auth memory service together with an advert
// memory service; both are seeded with the same user id so a token minted by
// auth authenticates against an advert-visible owner record.
type advertTestEnv struct {
	authSvc     *appauth.Service
	advertStore *appadvert.MemoryStore
	category    uuid.UUID
	district    uuid.UUID
	do          func(method, path, body, auth string) *httptest.ResponseRecorder
}

func newAdvertEngine(t *testing.T) *advertTestEnv {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, _, _ := appauth.NewMemoryServiceForTest(t)

	advertStore := appadvert.NewMemoryStore()
	category := uuid.New()
	district := uuid.New()
	advertStore.PutCategory(
		domaincatalog.Category{ID: category, Slug: "yarim-kan", Name: "Yarım Kan", IsActive: true},
		0,
		nil,
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
		req.Header.Set("X-Request-ID", "advert-http-1")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	return &advertTestEnv{authSvc: authSvc, advertStore: advertStore, category: category, district: district, do: do}
}

// registerAndLogin registers and logs a fresh user in via the real HTTP auth
// flow, then seeds the advert store's own user record with a matching id so
// the advert service's owner lookups (active account) resolve.
func (env *advertTestEnv) registerAndLogin(t *testing.T, email string, emailVerified bool) (bearer string, ownerID uuid.UUID) {
	t.Helper()
	rec := env.do(http.MethodPost, "/api/v1/auth/register",
		`{"email":"`+email+`","password":"Password1","firstName":"Ada","lastName":"Lovelace"}`, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register=%d %s", rec.Code, rec.Body.String())
	}
	rec = env.do(http.MethodPost, "/api/v1/auth/login",
		`{"email":"`+email+`","password":"Password1","clientContext":"PUBLIC_WEB"}`, "")
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
	var verifiedAt *time.Time
	if emailVerified {
		now := time.Now().UTC()
		verifiedAt = &now
	}
	env.advertStore.PutUser(domainuser.User{
		ID:              principal.UserID,
		Status:          domainuser.StatusActive,
		EmailVerifiedAt: verifiedAt,
	})
	return "Bearer " + tokens.AccessToken, principal.UserID
}

func TestCreateAdvertDraftHTTP(t *testing.T) {
	env := newAdvertEngine(t)
	auth, _ := env.registerAndLogin(t, "create@example.com", true)

	rec := env.do(http.MethodPost, "/api/v1/me/adverts", `{"title":"Satılık kısrak"}`, auth)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.OwnerAdvertResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Title == nil || *body.Title != "Satılık kısrak" {
		t.Fatalf("%+v", body)
	}
	if body.Status != generated.DRAFT || body.Version != 1 {
		t.Fatalf("%+v", body)
	}
	if body.Media == nil || len(body.Media) != 0 {
		t.Fatalf("media must be an empty non-nil slice, got %#v", body.Media)
	}
	if !strings.Contains(rec.Body.String(), `"media":[]`) {
		t.Fatalf("media must serialize as [], got %s", rec.Body.String())
	}
	if body.Properties == nil {
		t.Fatalf("properties must not be nil, got %#v", body.Properties)
	}
}

func TestListMyAdvertsEmptyHTTP(t *testing.T) {
	env := newAdvertEngine(t)
	auth, _ := env.registerAndLogin(t, "list@example.com", true)

	rec := env.do(http.MethodGet, "/api/v1/me/adverts", "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("empty list must serialize items as [], got %s", rec.Body.String())
	}
	var body generated.OwnerAdvertListResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.HasMore {
		t.Fatalf("%+v", body)
	}
}

func TestGetMyAdvertHTTP(t *testing.T) {
	env := newAdvertEngine(t)
	auth, _ := env.registerAndLogin(t, "get@example.com", true)

	rec := env.do(http.MethodPost, "/api/v1/me/adverts", `{}`, auth)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created generated.OwnerAdvertResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = env.do(http.MethodGet, "/api/v1/me/adverts/"+created.Id.String(), "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got generated.OwnerAdvertResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Id != created.Id {
		t.Fatalf("%+v", got)
	}
}

func TestGetMyAdvertNotFoundHTTP(t *testing.T) {
	env := newAdvertEngine(t)
	auth, _ := env.registerAndLogin(t, "notfound@example.com", true)

	rec := env.do(http.MethodGet, "/api/v1/me/adverts/"+uuid.New().String(), "", auth)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Code != generated.DomainErrorCodeNOTFOUND {
		t.Fatalf("%+v", errBody)
	}
}

func TestAdvertMissingAuthHTTP(t *testing.T) {
	env := newAdvertEngine(t)

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/me/adverts"},
		{http.MethodGet, "/api/v1/me/adverts"},
		{http.MethodGet, "/api/v1/me/adverts/" + uuid.New().String()},
	} {
		rec := env.do(tc.method, tc.path, "", "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status=%d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestGetMyAdvertMalformedUUIDHTTP(t *testing.T) {
	env := newAdvertEngine(t)
	auth, _ := env.registerAndLogin(t, "malformed@example.com", true)

	rec := env.do(http.MethodGet, "/api/v1/me/adverts/not-a-uuid", "", auth)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Code != generated.DomainErrorCodeVALIDATIONERROR {
		t.Fatalf("%+v", errBody)
	}
}

func TestUpdateAdvertDraftDetailsHTTP(t *testing.T) {
	env := newAdvertEngine(t)
	auth, _ := env.registerAndLogin(t, "update@example.com", true)

	rec := env.do(http.MethodPost, "/api/v1/me/adverts", `{"title":"Eski başlık"}`, auth)
	var created generated.OwnerAdvertResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = env.do(http.MethodPatch, "/api/v1/me/adverts/"+created.Id.String(),
		`{"title":"Yeni başlık","expectedVersion":1}`, auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var updated generated.OwnerAdvertResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Title == nil || *updated.Title != "Yeni başlık" {
		t.Fatalf("%+v", updated)
	}
	if updated.Version != 2 {
		t.Fatalf("version=%d", updated.Version)
	}
}

func TestUpdateAdvertDraftDetailsMalformedHTTP(t *testing.T) {
	env := newAdvertEngine(t)
	auth, _ := env.registerAndLogin(t, "malformedbody@example.com", true)

	rec := env.do(http.MethodPost, "/api/v1/me/adverts", `{}`, auth)
	var created generated.OwnerAdvertResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = env.do(http.MethodPatch, "/api/v1/me/adverts/"+created.Id.String(), `{"title":`, auth)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Code != generated.DomainErrorCodeVALIDATIONERROR {
		t.Fatalf("%+v", errBody)
	}
}

func TestSubmitAdvertForReviewUnverifiedEmailHTTP(t *testing.T) {
	env := newAdvertEngine(t)
	auth, _ := env.registerAndLogin(t, "unverified@example.com", false)

	rec := env.do(http.MethodPost, "/api/v1/me/adverts", `{}`, auth)
	var created generated.OwnerAdvertResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = env.do(http.MethodPost, "/api/v1/me/adverts/"+created.Id.String()+"/submit", `{"expectedVersion":1}`, auth)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("unverified owner must not be blocked on submit: %s", rec.Body.String())
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if errBody.Code != generated.DomainErrorCodeVALIDATIONERROR {
		t.Fatalf("%+v", errBody)
	}
}

func TestSoftDeleteAdvertDraftHTTP(t *testing.T) {
	env := newAdvertEngine(t)
	auth, _ := env.registerAndLogin(t, "delete@example.com", true)

	rec := env.do(http.MethodPost, "/api/v1/me/adverts", `{}`, auth)
	var created generated.OwnerAdvertResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = env.do(http.MethodDelete, "/api/v1/me/adverts/"+created.Id.String()+"?expectedVersion=1", "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var deleted generated.OwnerAdvertResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &deleted)
	if deleted.DeletedAt == nil {
		t.Fatalf("%+v", deleted)
	}

	rec = env.do(http.MethodGet, "/api/v1/me/adverts/"+created.Id.String(), "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("get-after-delete status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPublicAndOutOfScopeAdvertRoutesStill501HTTP(t *testing.T) {
	env := newAdvertEngine(t)
	auth, _ := env.registerAndLogin(t, "outofscope@example.com", true)

	rec := env.do(http.MethodGet, "/api/v1/adverts", "", "")
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("search published status=%d body=%s", rec.Code, rec.Body.String())
	}

	advertID := uuid.New()
	rec = env.do(http.MethodPost, "/api/v1/me/adverts/"+advertID.String()+"/media",
		`{"assetId":"`+uuid.New().String()+`","expectedMediaVersion":1}`, auth)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("media attach status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodGet, "/api/v1/me/favorites", "", auth)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("favorites status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReplaceAdvertDynamicPropertiesSellerPhoneHTTP(t *testing.T) {
	env := newAdvertEngine(t)
	auth, _ := env.registerAndLogin(t, "sellerphone@example.com", true)

	rec := env.do(http.MethodPost, "/api/v1/me/adverts",
		`{"categoryId":"`+env.category.String()+`","districtId":"`+env.district.String()+`","title":"Telefon Testi İlanı","price":{"amountMinor":100000,"currency":"TRY"}}`, auth)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created generated.OwnerAdvertResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = env.do(http.MethodPut, "/api/v1/me/adverts/"+created.Id.String()+"/properties",
		`{"expectedVersion":1,"properties":{"sellerPhone":"+90 532 999 88 77","phone":"+90 532 999 88 77"}}`, auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("put properties status=%d body=%s", rec.Code, rec.Body.String())
	}
	var updated generated.OwnerAdvertResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Version != 2 {
		t.Fatalf("expected version 2, got %d", updated.Version)
	}

	phoneVal, ok := updated.Properties["sellerPhone"]
	if !ok || phoneVal != "+90 532 999 88 77" {
		t.Fatalf("sellerPhone not set: %+v", updated.Properties)
	}
}

