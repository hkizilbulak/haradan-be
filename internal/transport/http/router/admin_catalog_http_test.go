package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminCatalogRoutesRequireAuthentication(t *testing.T) {
	engine := newTestEngine(&geoRepoStub{}, &catalogRepoStub{})
	for _, path := range []string{
		"/api/v1/admin/categories",
		"/api/v1/admin/categories/11111111-1111-1111-1111-111111111111",
		"/api/v1/admin/categories/11111111-1111-1111-1111-111111111111/properties",
	} {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: expected 401, got %d", path, rec.Code)
		}
	}
}
