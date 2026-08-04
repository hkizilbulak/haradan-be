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

	appcatalog "github.com/hkizilbulak/haradan-be/internal/application/catalog"
	appgeo "github.com/hkizilbulak/haradan-be/internal/application/geo"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

type geoRepoStub struct {
	provinces   []domaingeo.Province
	districts   []domaingeo.District
	provinceOK  map[uuid.UUID]bool
	listErr     error
	internalErr error
}

func (g *geoRepoStub) ListActiveProvinces(context.Context) ([]domaingeo.Province, error) {
	if g.listErr != nil {
		return nil, g.listErr
	}
	return g.provinces, nil
}
func (g *geoRepoStub) SearchActiveProvincesByNormalizedPrefix(context.Context, string, int) ([]domaingeo.Province, error) {
	if g.internalErr != nil {
		return nil, g.internalErr
	}
	return g.provinces, nil
}
func (g *geoRepoStub) GetActiveProvinceID(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	if g.provinceOK != nil && !g.provinceOK[id] {
		return uuid.Nil, apperr.NotFound("İl bulunamadı.")
	}
	return id, nil
}
func (g *geoRepoStub) GetActiveDistrict(_ context.Context, id uuid.UUID) (domaingeo.District, error) {
	for _, d := range g.districts {
		if d.ID == id && d.IsActive {
			return d, nil
		}
	}
	return domaingeo.District{}, apperr.NotFound("İlçe bulunamadı.")
}
func (g *geoRepoStub) ListActiveDistrictsByProvince(_ context.Context, provinceID uuid.UUID) ([]domaingeo.District, error) {
	var out []domaingeo.District
	for _, d := range g.districts {
		if d.ProvinceID == provinceID {
			out = append(out, d)
		}
	}
	return out, nil
}
func (g *geoRepoStub) SearchActiveDistrictsByNormalizedPrefix(context.Context, string, *uuid.UUID, int) ([]domaingeo.District, error) {
	return g.districts, nil
}

type catalogRepoStub struct {
	categories []domaincatalog.Category
	props      map[uuid.UUID][]domaincatalog.Property
	children   map[uuid.UUID]int
}

func (c *catalogRepoStub) ListActiveCategories(context.Context) ([]domaincatalog.Category, error) {
	return c.categories, nil
}
func (c *catalogRepoStub) GetActiveCategory(_ context.Context, id uuid.UUID) (domaincatalog.Category, error) {
	for _, cat := range c.categories {
		if cat.ID == id {
			return cat, nil
		}
	}
	return domaincatalog.Category{}, apperr.NotFound("Kategori bulunamadı.")
}
func (c *catalogRepoStub) CountActiveChildren(_ context.Context, parentID uuid.UUID) (int, error) {
	return c.children[parentID], nil
}
func (c *catalogRepoStub) ListFormProperties(_ context.Context, categoryID uuid.UUID) ([]domaincatalog.Property, error) {
	return c.props[categoryID], nil
}

func newTestEngine(geoRepo *geoRepoStub, catalogRepo *catalogRepoStub) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	geoSvc := appgeo.NewService(geoRepo)
	catalogSvc := appcatalog.NewService(catalogRepo)
	srv := handler.NewServer(log, fakeDeps{}, geoSvc, catalogSvc, nil, nil, nil, nil)
	return router.New(srv, log)
}

func TestListActiveProvincesHTTP(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	engine := newTestEngine(&geoRepoStub{
		provinces: []domaingeo.Province{{ID: id, Name: "Ankara", SortOrder: 1}},
	}, &catalogRepoStub{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces", nil)
	req.Header.Set("X-Request-ID", "geo-list-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Request-ID") != "geo-list-1" {
		t.Fatalf("request id=%q", rec.Header().Get("X-Request-ID"))
	}
	var body generated.ProvinceListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].Name != "Ankara" {
		t.Fatalf("body=%+v", body)
	}
}

func TestListActiveProvincesEmptyList(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	raw := rec.Body.String()
	if strings.Contains(raw, `"items":null`) || !strings.Contains(raw, `"items":[]`) {
		t.Fatalf("empty list must be [], got %s", raw)
	}
	var body generated.ProvinceListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Items == nil || len(body.Items) != 0 {
		t.Fatalf("items=%v", body.Items)
	}
}

func TestSearchProvincesLimitUpperBound(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces/search?q=ank&limit=101", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != generated.DomainErrorCodeVALIDATIONERROR {
		t.Fatalf("code=%q", body.Code)
	}
}

func TestSearchDistrictsWhitespaceQueryEmptyArray(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{provinceOK: map[uuid.UUID]bool{}}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/districts/search?q=%20%20", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestCategoryTreeEmptyItemsArray(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestSearchProvincesInvalidLimit(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces/search?q=ank&limit=0", nil)
	req.Header.Set("X-Request-ID", "geo-val-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != generated.DomainErrorCodeVALIDATIONERROR || body.TraceId != "geo-val-1" {
		t.Fatalf("body=%+v", body)
	}
}

func TestListDistrictsNotFound(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{provinceOK: map[uuid.UUID]bool{}}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces/22222222-2222-2222-2222-222222222222/districts", nil)
	req.Header.Set("X-Request-ID", "geo-nf-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != generated.DomainErrorCodeNOTFOUND || body.TraceId != "geo-nf-1" {
		t.Fatalf("body=%+v", body)
	}
}

func TestInvalidProvincePathUUID(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces/not-a-uuid/districts", nil)
	req.Header.Set("X-Request-ID", "geo-bad-uuid-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body generated.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != generated.DomainErrorCodeVALIDATIONERROR || body.TraceId != "geo-bad-uuid-1" {
		t.Fatalf("body=%+v", body)
	}
	if strings.Contains(rec.Body.String(), "Invalid format") {
		t.Fatalf("leaked parser message: %s", rec.Body.String())
	}
}

func TestSearchProvincesInternalErrorNoLeak(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{
		internalErr: errors.New("dial tcp password=super-secret"),
	}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces/search?q=ank", nil)
	req.Header.Set("X-Request-ID", "geo-int-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "super-secret") || strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("leaked secret: %s", rec.Body.String())
	}
	var body generated.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != generated.DomainErrorCodeINTERNALERROR || body.TraceId != "geo-int-1" {
		t.Fatalf("body=%+v", body)
	}
}

func TestCategoryTreeAndFormHTTP(t *testing.T) {
	leaf := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	parent := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{
		categories: []domaincatalog.Category{
			{ID: parent, Slug: "parent", Name: "Parent", SortOrder: 1},
			{ID: leaf, ParentID: &parent, Slug: "leaf", Name: "Leaf", SortOrder: 1},
		},
		children: map[uuid.UUID]int{parent: 1, leaf: 0},
		props: map[uuid.UUID][]domaincatalog.Property{
			leaf: {{
				ID: uuid.New(), CategoryID: leaf, Code: "color", Title: "Renk",
				DataType: "STRING", SortOrder: 1, Options: json.RawMessage(`[{"value":"red"}]`),
				UIMetadata: json.RawMessage(`{"widget":"select"}`),
			}},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tree status=%d body=%s", rec.Code, rec.Body.String())
	}
	var tree generated.CategoryTreeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	if len(tree.Items) != 1 || len(tree.Items[0].Children) != 1 {
		t.Fatalf("tree=%+v", tree)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/categories/"+leaf.String()+"/form", nil)
	req.Header.Set("X-Request-ID", "cat-form-1")
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("form status=%d body=%s", rec.Code, rec.Body.String())
	}
	var form generated.CategoryFormDefinitionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &form); err != nil {
		t.Fatal(err)
	}
	if form.Slug != "leaf" || len(form.Properties) != 1 || form.Properties[0].Code != "color" {
		t.Fatalf("form=%+v", form)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/categories/"+parent.String()+"/form", nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("parent form status=%d body=%s", rec.Code, rec.Body.String())
	}
	var errBody generated.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Code != generated.DomainErrorCodeINVALIDSTATE {
		t.Fatalf("code=%q", errBody.Code)
	}
}

func TestGeoCatalogNoLonger501(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	paths := []string{
		"/api/v1/provinces",
		"/api/v1/provinces/search?q=a",
		"/api/v1/districts/search?q=a",
		"/api/v1/categories",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotImplemented {
			t.Fatalf("%s still 501", path)
		}
	}
}

func TestOutOfScopeStill501(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/horses", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status=%d", rec.Code)
	}
}
