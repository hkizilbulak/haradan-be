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
	appfavorite "github.com/hkizilbulak/haradan-be/internal/application/favorite"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

type favoriteTestEnv struct {
	authSvc *appauth.Service
	store   *appfavorite.MemoryStore
	do      func(method, path, body, auth string) *httptest.ResponseRecorder
}

func newFavoriteEngine(t *testing.T) *favoriteTestEnv {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, _, _ := appauth.NewMemoryServiceForTest(t)
	store := appfavorite.NewMemoryStore()
	favSvc, err := appfavorite.NewMemoryService(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := handler.NewServer(log, fakeDeps{}, nil, nil, nil, nil, nil, favSvc, nil, nil, nil, nil, authSvc)
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
		req.Header.Set("X-Request-ID", "favorite-http-1")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	return &favoriteTestEnv{authSvc: authSvc, store: store, do: do}
}

func (env *favoriteTestEnv) registerAndLogin(t *testing.T, email string) (string, uuid.UUID) {
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
	return "Bearer " + tokens.AccessToken, principal.UserID
}

func (env *favoriteTestEnv) seedPublished(id uuid.UUID) {
	title := "Favori ilan"
	now := time.Now().UTC()
	cat := uuid.New()
	district := uuid.New()
	province := uuid.New()
	amount := int64(1000)
	currency := "TRY"
	env.store.PutAdvert(appfavorite.AdvertSnapshot{
		ID: id, Status: string(domainadvert.StatusPublished),
		Title: &title, PublishedAt: &now, CategoryID: &cat, DistrictID: &district, ProvinceID: &province,
		PriceAmountMinor: &amount, PriceCurrency: &currency,
	})
}

func TestFavoriteAddRemoveListHTTP(t *testing.T) {
	env := newFavoriteEngine(t)
	auth, _ := env.registerAndLogin(t, "fav-owner@example.com")
	advertID := uuid.New()
	env.seedPublished(advertID)

	rec := env.do(http.MethodPut, "/api/v1/me/favorites/"+advertID.String(), "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("add status=%d body=%s", rec.Code, rec.Body.String())
	}
	var mut generated.FavoriteMutationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &mut); err != nil {
		t.Fatal(err)
	}
	if !mut.Favorited || mut.AdvertId != advertID {
		t.Fatalf("%+v", mut)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type=%s", rec.Header().Get("Content-Type"))
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody) // mutation body has no TraceId; ensure success shape

	rec = env.do(http.MethodPut, "/api/v1/me/favorites/"+advertID.String(), "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("idempotent add=%d", rec.Code)
	}

	rec = env.do(http.MethodGet, "/api/v1/me/favorites", "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("list=%d %s", rec.Code, rec.Body.String())
	}
	var list generated.FavoriteListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || !list.Items[0].Available || list.Items[0].Card == nil {
		t.Fatalf("%+v", list)
	}
	if list.Items[0].Card.Cover != nil {
		t.Fatal("cover must be null without public URL projection")
	}
	raw := rec.Body.String()
	for _, leak := range []string{"ownerUserId", "deleted_at", "password", "moderation"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("leak %q in %s", leak, raw)
		}
	}

	rec = env.do(http.MethodDelete, "/api/v1/me/favorites/"+advertID.String(), "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("remove=%d", rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &mut)
	if mut.Favorited {
		t.Fatal("favorited must be false")
	}
	rec = env.do(http.MethodDelete, "/api/v1/me/favorites/"+advertID.String(), "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("idempotent remove=%d", rec.Code)
	}

	rec = env.do(http.MethodGet, "/api/v1/me/favorites", "", auth)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Items == nil || len(list.Items) != 0 {
		t.Fatalf("empty list=%+v", list)
	}
}

func TestFavoriteAuthAndValidationHTTP(t *testing.T) {
	env := newFavoriteEngine(t)
	advertID := uuid.New()
	env.seedPublished(advertID)

	rec := env.do(http.MethodPut, "/api/v1/me/favorites/"+advertID.String(), "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth=%d", rec.Code)
	}
	var body generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.TraceId != "favorite-http-1" {
		t.Fatalf("traceId=%q", body.TraceId)
	}

	rec = env.do(http.MethodPut, "/api/v1/me/favorites/"+advertID.String(), "", "Bearer not-a-token")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid bearer=%d", rec.Code)
	}

	rec = env.do(http.MethodPut, "/api/v1/me/favorites/not-a-uuid", "", "Bearer x")
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnauthorized {
		// malformed UUID is rejected by generated binder as 400; invalid bearer may win first
		t.Fatalf("malformed uuid status=%d", rec.Code)
	}

	auth, _ := env.registerAndLogin(t, "fav-val@example.com")
	rec = env.do(http.MethodPut, "/api/v1/me/favorites/not-a-uuid", "", auth)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed uuid authenticated=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodPut, "/api/v1/me/favorites/"+uuid.New().String(), "", auth)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing advert=%d body=%s", rec.Code, rec.Body.String())
	}
	var missingBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &missingBody)

	draftID := uuid.New()
	env.store.PutAdvert(appfavorite.AdvertSnapshot{ID: draftID, Status: string(domainadvert.StatusDraft)})
	rec = env.do(http.MethodPut, "/api/v1/me/favorites/"+draftID.String(), "", auth)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("draft add=%d body=%s", rec.Code, rec.Body.String())
	}
	var draftBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &draftBody)
	if draftBody.Code != generated.DomainErrorCodeNOTFOUND || draftBody.Code != missingBody.Code {
		t.Fatalf("draft code=%s missing code=%s", draftBody.Code, missingBody.Code)
	}
	if draftBody.Message != missingBody.Message {
		t.Fatalf("messages differ: %q vs %q", draftBody.Message, missingBody.Message)
	}

	deletedID := uuid.New()
	now := time.Now().UTC()
	title := "Silindi"
	cat := uuid.New()
	district := uuid.New()
	province := uuid.New()
	env.store.PutAdvert(appfavorite.AdvertSnapshot{
		ID: deletedID, Status: string(domainadvert.StatusPublished), DeletedAt: &now,
		Title: &title, PublishedAt: &now, CategoryID: &cat, DistrictID: &district, ProvinceID: &province,
	})
	rec = env.do(http.MethodPut, "/api/v1/me/favorites/"+deletedID.String(), "", auth)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("soft-deleted add=%d body=%s", rec.Code, rec.Body.String())
	}
	var deletedBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &deletedBody)
	if deletedBody.Code != missingBody.Code || deletedBody.Message != missingBody.Message {
		t.Fatalf("soft-deleted distinguishable: %+v vs %+v", deletedBody, missingBody)
	}
}

func TestFavoriteCrossUserIsolationHTTP(t *testing.T) {
	env := newFavoriteEngine(t)
	ownerAuth, _ := env.registerAndLogin(t, "fav-a@example.com")
	otherAuth, _ := env.registerAndLogin(t, "fav-b@example.com")
	advertID := uuid.New()
	env.seedPublished(advertID)

	rec := env.do(http.MethodPut, "/api/v1/me/favorites/"+advertID.String(), "", ownerAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner add=%d", rec.Code)
	}
	rec = env.do(http.MethodGet, "/api/v1/me/favorites", "", otherAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("other list=%d", rec.Code)
	}
	var list generated.FavoriteListResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Items) != 0 {
		t.Fatalf("cross-user leak=%+v", list)
	}
	rec = env.do(http.MethodDelete, "/api/v1/me/favorites/"+advertID.String(), "", otherAuth)
	if rec.Code != http.StatusOK {
		t.Fatalf("other delete=%d", rec.Code)
	}
	rec = env.do(http.MethodGet, "/api/v1/me/favorites", "", ownerAuth)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Items) != 1 {
		t.Fatalf("owner favorite removed by stranger: %+v", list)
	}
}

func TestFavoriteUnavailablePlaceholderHTTP(t *testing.T) {
	env := newFavoriteEngine(t)
	auth, _ := env.registerAndLogin(t, "fav-ph@example.com")
	advertID := uuid.New()
	env.seedPublished(advertID)
	rec := env.do(http.MethodPut, "/api/v1/me/favorites/"+advertID.String(), "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("add=%d", rec.Code)
	}
	env.store.PutAdvert(appfavorite.AdvertSnapshot{ID: advertID, Status: string(domainadvert.StatusSold)})

	rec = env.do(http.MethodGet, "/api/v1/me/favorites", "", auth)
	var list generated.FavoriteListResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Items) != 1 || list.Items[0].Available || list.Items[0].Card != nil {
		t.Fatalf("%+v", list)
	}
	if list.Items[0].UnavailableReason == nil {
		t.Fatal("placeholder reason required")
	}
	var generic map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &generic); err != nil {
		t.Fatal(err)
	}
	items, _ := generic["items"].([]any)
	item, _ := items[0].(map[string]any)
	if _, ok := item["card"]; ok && item["card"] != nil {
		t.Fatalf("card present: %+v", item["card"])
	}
	allowed := map[string]struct{}{"advertId": {}, "available": {}, "unavailableReason": {}}
	for key := range item {
		if _, ok := allowed[key]; !ok {
			t.Fatalf("unexpected placeholder field %q in %v", key, item)
		}
	}
}

func TestFavoriteOpsNoLonger501HTTP(t *testing.T) {
	env := newFavoriteEngine(t)
	auth, _ := env.registerAndLogin(t, "fav-501@example.com")
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/me/favorites"},
		{http.MethodPut, "/api/v1/me/favorites/" + uuid.New().String()},
		{http.MethodDelete, "/api/v1/me/favorites/" + uuid.New().String()},
	} {
		rec := env.do(tc.method, tc.path, "", auth)
		if rec.Code == http.StatusNotImplemented {
			t.Fatalf("%s %s still 501", tc.method, tc.path)
		}
	}
}

func TestFavoriteOutOfScopeStill501HTTP(t *testing.T) {
	env := newFavoriteEngine(t)
	auth, _ := env.registerAndLogin(t, "fav-oos@example.com")
	rec := env.do(http.MethodGet, "/api/v1/adverts", "", "")
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("public search=%d", rec.Code)
	}
	rec = env.do(http.MethodPost, "/api/v1/admin/media/uploads", `{}`, auth)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("admin media=%d", rec.Code)
	}
}
