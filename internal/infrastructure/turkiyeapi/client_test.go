package turkiyeapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/turkiyeapi"
)

func TestFetchCatalogMapsOfficialIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/provinces" || r.URL.Query().Get("limit") != "81" {
			t.Errorf("path=%s query=%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "OK",
			"data":   validCatalogPayload(),
		})
	}))
	defer srv.Close()

	client, err := turkiyeapi.New(turkiyeapi.Config{BaseURL: srv.URL, HTTPTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Provinces) != 81 || len(cat.Districts) != 81 {
		t.Fatalf("provinces=%d districts=%d", len(cat.Provinces), len(cat.Districts))
	}
	adana := cat.Provinces[0]
	if adana.ID != domaingeo.StableProvinceID(1) || adana.Name != "Adana" || adana.SortOrder != 1 {
		t.Fatalf("adana=%+v", adana)
	}
	if cat.Districts[0].ID != domaingeo.StableDistrictID(1001) {
		t.Fatalf("district id=%s", cat.Districts[0].ID)
	}
}

func TestFetchCatalogRejectsIncompletePayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "OK",
			"data":   []map[string]any{{"id": 1, "name": "Adana", "districts": []map[string]any{{"id": 1, "name": "Seyhan"}}}},
		})
	}))
	defer srv.Close()

	client, err := turkiyeapi.New(turkiyeapi.Config{BaseURL: srv.URL, HTTPTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.FetchCatalog(context.Background())
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("want dependency unavailable, got %v", err)
	}
}

func TestFetchCatalogHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	client, err := turkiyeapi.New(turkiyeapi.Config{BaseURL: srv.URL, HTTPTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.FetchCatalog(context.Background())
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("want dependency unavailable, got %v", err)
	}
}

func validCatalogPayload() []map[string]any {
	out := make([]map[string]any, 0, 81)
	for i := 1; i <= 81; i++ {
		out = append(out, map[string]any{
			"id":   i,
			"name": provinceName(i),
			"districts": []map[string]any{
				{"id": 1000 + i, "name": "Merkez"},
			},
		})
	}
	return out
}

func provinceName(i int) string {
	if i == 1 {
		return "Adana"
	}
	return "İl"
}
