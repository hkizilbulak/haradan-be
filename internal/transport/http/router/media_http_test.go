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
	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

// mediaTestEnv wires a real auth memory service together with a media memory
// service backed by FakeStorage/FakeProcessor so MEDIA-01..07 can be driven
// end-to-end without a database, an object store or a compression provider.
type mediaTestEnv struct {
	authSvc     *appauth.Service
	mediaStore  *appmedia.MemoryStore
	fakeStorage *appmedia.FakeStorage
	do          func(method, path, body, auth string) *httptest.ResponseRecorder
}

const (
	mediaAllowedContentType = "image/png"
	mediaMaxByteSize        = 10 * 1024 * 1024
)

func newMediaEngine(t *testing.T) *mediaTestEnv {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, _, _ := appauth.NewMemoryServiceForTest(t)

	store := appmedia.NewMemoryStore()
	fakeStorage := appmedia.NewFakeStorage(nil)
	mediaSvc, err := appmedia.NewMemoryService(store, nil, appmedia.Config{
		Storage:             fakeStorage,
		Processor:           appmedia.FakeProcessor{},
		AllowedContentTypes: []string{mediaAllowedContentType, "image/jpeg"},
		MaxByteSize:         mediaMaxByteSize,
		UploadURLTTL:        15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := handler.NewServer(log, fakeDeps{}, nil, nil, nil, nil, mediaSvc, nil, nil, nil, nil, nil, authSvc)
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
		req.Header.Set("X-Request-ID", "media-http-1")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	return &mediaTestEnv{authSvc: authSvc, mediaStore: store, fakeStorage: fakeStorage, do: do}
}

// registerAndLogin registers and logs a fresh user in via the real HTTP auth
// flow and returns the bearer token plus the resolved owner id.
func (env *mediaTestEnv) registerAndLogin(t *testing.T, email string) (bearer string, ownerID uuid.UUID) {
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

// initiateAsset drives MEDIA-01 over HTTP and returns the created asset id.
func (env *mediaTestEnv) initiateAsset(t *testing.T, auth string) uuid.UUID {
	t.Helper()
	rec := env.do(http.MethodPost, "/api/v1/media/uploads",
		`{"declaredContentType":"image/png","declaredByteSize":1024}`, auth)
	if rec.Code != http.StatusCreated {
		t.Fatalf("initiate status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.InitiateMediaUploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.AssetId
}

// putRawUpload simulates the client's direct PUT to the storage provider by
// writing straight into the fake object store the test env holds.
func (env *mediaTestEnv) putRawUpload(t *testing.T, assetID uuid.UUID) {
	t.Helper()
	if err := env.fakeStorage.PutObject(
		context.Background(),
		domainmedia.RawObjectKey(assetID),
		mediaAllowedContentType,
		appmedia.FakeImageBytes(),
	); err != nil {
		t.Fatal(err)
	}
}

// draftAdvert seeds a DRAFT advert directly in the media store, matching what
// the advert domain would have produced, so MEDIA-04..07 guards resolve.
func (env *mediaTestEnv) draftAdvert(ownerID uuid.UUID) uuid.UUID {
	advertID := uuid.New()
	env.mediaStore.PutAdvert(appmedia.MemoryAdvert{
		ID:           advertID,
		OwnerUserID:  ownerID,
		Status:       "DRAFT",
		MediaVersion: 1,
	})
	return advertID
}

func TestInitiateMediaUploadHTTP(t *testing.T) {
	env := newMediaEngine(t)
	auth, _ := env.registerAndLogin(t, "initiate@example.com")

	rec := env.do(http.MethodPost, "/api/v1/media/uploads",
		`{"declaredContentType":"image/png","declaredByteSize":2048}`, auth)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.InitiateMediaUploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.AssetId == uuid.Nil {
		t.Fatalf("%+v", body)
	}
	if body.Upload.Method != generated.PUT || body.Upload.Url == "" {
		t.Fatalf("%+v", body.Upload)
	}
	if len(body.Constraints.AllowedContentTypes) != 2 || body.Constraints.MaxByteSize != mediaMaxByteSize {
		t.Fatalf("%+v", body.Constraints)
	}
	// The upload URL necessarily encodes where to PUT the file (that is how a
	// presigned/direct-PUT upload grant works); what must never appear is the
	// object-key field itself or the storage provider name.
	raw := rec.Body.String()
	for _, leak := range []string{"objectKey", "rawObjectKey", "masterObjectKey", `"provider"`, "B2"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("leak %q in %s", leak, raw)
		}
	}
}

func TestInitiateMediaUploadMalformedBodyHTTP(t *testing.T) {
	env := newMediaEngine(t)
	auth, _ := env.registerAndLogin(t, "malformed@example.com")

	rec := env.do(http.MethodPost, "/api/v1/media/uploads", `{"declaredContentType":`, auth)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Code != generated.DomainErrorCodeVALIDATIONERROR {
		t.Fatalf("%+v", errBody)
	}
}

func TestInitiateMediaUploadDependencyUnavailableHTTP(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, _, _ := appauth.NewMemoryServiceForTest(t)
	store := appmedia.NewMemoryStore()
	// Deliberately unconfigured: no allowed content types, no byte ceiling.
	mediaSvc, err := appmedia.NewMemoryService(store, nil, appmedia.Config{})
	if err != nil {
		t.Fatal(err)
	}
	srv := handler.NewServer(log, fakeDeps{}, nil, nil, nil, nil, mediaSvc, nil, nil, nil, nil, nil, authSvc)
	engine := router.New(srv, log, router.Options{AuthService: authSvc})

	env := &mediaTestEnv{authSvc: authSvc, do: func(method, path, body, auth string) *httptest.ResponseRecorder {
		var rdr io.Reader
		if body != "" {
			rdr = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, rdr)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}}
	auth, _ := env.registerAndLogin(t, "unconfigured@example.com")

	rec := env.do(http.MethodPost, "/api/v1/media/uploads", `{}`, auth)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Code != generated.DomainErrorCodeDEPENDENCYUNAVAILABLE {
		t.Fatalf("%+v", errBody)
	}
}

func TestConfirmMediaUploadHTTP(t *testing.T) {
	env := newMediaEngine(t)
	auth, _ := env.registerAndLogin(t, "confirm@example.com")
	assetID := env.initiateAsset(t, auth)
	env.putRawUpload(t, assetID)

	rec := env.do(http.MethodPost, "/api/v1/media/assets/"+assetID.String()+"/confirm", `{}`, auth)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.MediaProcessingState
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.AssetId != assetID {
		t.Fatalf("%+v", body)
	}
	if body.LifecycleStatus != generated.MediaAssetLifecycleUPLOADED {
		t.Fatalf("lifecycleStatus=%s", body.LifecycleStatus)
	}
}

func TestConfirmMediaUploadWithoutUploadHTTP(t *testing.T) {
	env := newMediaEngine(t)
	auth, _ := env.registerAndLogin(t, "notuploaded@example.com")
	assetID := env.initiateAsset(t, auth)

	rec := env.do(http.MethodPost, "/api/v1/media/assets/"+assetID.String()+"/confirm", `{}`, auth)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetMediaProcessingStatusHTTP(t *testing.T) {
	env := newMediaEngine(t)
	auth, _ := env.registerAndLogin(t, "status@example.com")
	assetID := env.initiateAsset(t, auth)
	env.putRawUpload(t, assetID)
	if rec := env.do(http.MethodPost, "/api/v1/media/assets/"+assetID.String()+"/confirm", `{}`, auth); rec.Code != http.StatusAccepted {
		t.Fatalf("confirm status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec := env.do(http.MethodGet, "/api/v1/media/assets/"+assetID.String(), "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.MediaProcessingState
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.LifecycleStatus != generated.MediaAssetLifecycleUPLOADED {
		t.Fatalf("lifecycleStatus=%s", body.LifecycleStatus)
	}
	if body.Variants == nil {
		t.Fatalf("variants must be a non-nil (possibly empty) slice, got %#v", body.Variants)
	}
	for _, v := range body.Variants {
		if v.PublicUrl != nil {
			t.Fatalf("publicUrl must stay nil, got %+v", v)
		}
	}
}

func TestGetMediaProcessingStatusNotFoundHTTP(t *testing.T) {
	env := newMediaEngine(t)
	auth, _ := env.registerAndLogin(t, "statusnotfound@example.com")

	rec := env.do(http.MethodGet, "/api/v1/media/assets/"+uuid.New().String(), "", auth)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAttachReorderCoverDetachAdvertMediaHTTP(t *testing.T) {
	env := newMediaEngine(t)
	auth, ownerID := env.registerAndLogin(t, "owner@example.com")
	advertID := env.draftAdvert(ownerID)

	asset1 := env.initiateAsset(t, auth)
	asset2 := env.initiateAsset(t, auth)

	// Attach asset1.
	rec := env.do(http.MethodPost, "/api/v1/me/adverts/"+advertID.String()+"/media",
		`{"assetId":"`+asset1.String()+`","expectedMediaVersion":1}`, auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("attach1 status=%d body=%s", rec.Code, rec.Body.String())
	}
	var collection generated.AdvertMediaCollectionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &collection)
	if collection.MediaVersion != 2 || len(collection.Items) != 1 || collection.Items[0].AssetId != asset1 {
		t.Fatalf("%+v", collection)
	}

	// Attach asset2.
	rec = env.do(http.MethodPost, "/api/v1/me/adverts/"+advertID.String()+"/media",
		`{"assetId":"`+asset2.String()+`","expectedMediaVersion":2}`, auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("attach2 status=%d body=%s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &collection)
	if collection.MediaVersion != 3 || len(collection.Items) != 2 {
		t.Fatalf("%+v", collection)
	}

	// Reorder: asset2 first, asset1 second.
	rec = env.do(http.MethodPut, "/api/v1/me/adverts/"+advertID.String()+"/media/order",
		`{"orderedAssetIds":["`+asset2.String()+`","`+asset1.String()+`"],"expectedMediaVersion":3}`, auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("reorder status=%d body=%s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &collection)
	if collection.MediaVersion != 4 || len(collection.Items) != 2 {
		t.Fatalf("%+v", collection)
	}
	if collection.Items[0].AssetId != asset2 || collection.Items[0].DisplayOrder != 0 {
		t.Fatalf("%+v", collection.Items)
	}
	if collection.Items[1].AssetId != asset1 || collection.Items[1].DisplayOrder != 1 {
		t.Fatalf("%+v", collection.Items)
	}

	// Set cover to asset2.
	rec = env.do(http.MethodPut, "/api/v1/me/adverts/"+advertID.String()+"/media/cover",
		`{"assetId":"`+asset2.String()+`","expectedMediaVersion":4}`, auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("cover status=%d body=%s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &collection)
	if collection.MediaVersion != 5 {
		t.Fatalf("%+v", collection)
	}
	for _, item := range collection.Items {
		if item.AssetId == asset2 && !item.IsCover {
			t.Fatalf("asset2 should be cover: %+v", collection.Items)
		}
		if item.AssetId == asset1 && item.IsCover {
			t.Fatalf("asset1 must not be cover: %+v", collection.Items)
		}
	}

	// Detach asset1.
	rec = env.do(http.MethodDelete,
		"/api/v1/me/adverts/"+advertID.String()+"/media/"+asset1.String()+"?expectedMediaVersion=5", "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("detach status=%d body=%s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &collection)
	if collection.MediaVersion != 6 || len(collection.Items) != 1 || collection.Items[0].AssetId != asset2 {
		t.Fatalf("%+v", collection)
	}

	raw := rec.Body.String()
	for _, leak := range []string{"objectKey", "rawObjectKey", "masterObjectKey", "assets/", "B2", "publicUrl"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("leak %q in %s", leak, raw)
		}
	}
}

func TestAttachAdvertMediaStaleVersionHTTP(t *testing.T) {
	env := newMediaEngine(t)
	auth, ownerID := env.registerAndLogin(t, "stale@example.com")
	advertID := env.draftAdvert(ownerID)
	asset1 := env.initiateAsset(t, auth)

	rec := env.do(http.MethodPost, "/api/v1/me/adverts/"+advertID.String()+"/media",
		`{"assetId":"`+asset1.String()+`","expectedMediaVersion":99}`, auth)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if string(errBody.Code) != "STALE_VERSION" {
		t.Fatalf("%+v", errBody)
	}
}

func TestMediaMissingAuthHTTP(t *testing.T) {
	env := newMediaEngine(t)
	advertID := uuid.New()
	assetID := uuid.New()

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/media/uploads", `{}`},
		{http.MethodPost, "/api/v1/media/assets/" + assetID.String() + "/confirm", `{}`},
		{http.MethodGet, "/api/v1/media/assets/" + assetID.String(), ""},
		{http.MethodPost, "/api/v1/me/adverts/" + advertID.String() + "/media",
			`{"assetId":"` + assetID.String() + `","expectedMediaVersion":1}`},
		{http.MethodDelete, "/api/v1/me/adverts/" + advertID.String() + "/media/" + assetID.String() + "?expectedMediaVersion=1", ""},
		{http.MethodPut, "/api/v1/me/adverts/" + advertID.String() + "/media/order",
			`{"orderedAssetIds":["` + assetID.String() + `"],"expectedMediaVersion":1}`},
		{http.MethodPut, "/api/v1/me/adverts/" + advertID.String() + "/media/cover",
			`{"assetId":"` + assetID.String() + `","expectedMediaVersion":1}`},
	} {
		rec := env.do(tc.method, tc.path, tc.body, "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status=%d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestMediaOutOfScopeRoutesStill501HTTP(t *testing.T) {
	env := newMediaEngine(t)
	auth, _ := env.registerAndLogin(t, "outofscope@example.com")

	rec := env.do(http.MethodPost, "/api/v1/admin/media/uploads", `{}`, auth)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("admin initiate status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodGet, "/api/v1/admin/media/assets/"+uuid.New().String(), "", auth)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("admin status status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodPost, "/api/v1/admin/media/assets/"+uuid.New().String()+"/confirm", `{}`, auth)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("admin confirm status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodGet, "/api/v1/adverts", "", "")
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("public search status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodGet, "/api/v1/me/favorites", "", auth)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("favorites status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMediaResponseNoLeakHTTP(t *testing.T) {
	env := newMediaEngine(t)
	auth, ownerID := env.registerAndLogin(t, "noleak@example.com")
	advertID := env.draftAdvert(ownerID)
	assetID := env.initiateAsset(t, auth)

	rec := env.do(http.MethodPost, "/api/v1/me/adverts/"+advertID.String()+"/media",
		`{"assetId":"`+assetID.String()+`","expectedMediaVersion":1}`, auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	for _, leak := range []string{
		"password", "passwordHash", "securityStamp", "refreshToken",
		"objectKey", "rawObjectKey", "masterObjectKey", "assets/", "B2", "credential",
	} {
		if strings.Contains(raw, leak) {
			t.Fatalf("leak %q in %s", leak, raw)
		}
	}
}
