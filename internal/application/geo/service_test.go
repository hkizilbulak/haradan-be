package geo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	appgeo "github.com/hkizilbulak/haradan-be/internal/application/geo"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
)

type fakeGeoRepo struct {
	provinces      []domaingeo.Province
	districts      []domaingeo.District
	provinceIDs    map[uuid.UUID]bool
	listErr        error
	searchProvErr  error
	getProvErr     error
	listDistErr    error
	searchDistErr  error
	lastProvPrefix string
	lastDistPrefix string
	lastLimit      int
}

func (f *fakeGeoRepo) ListActiveProvinces(context.Context) ([]domaingeo.Province, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]domaingeo.Province(nil), f.provinces...), nil
}

func (f *fakeGeoRepo) SearchActiveProvincesByNormalizedPrefix(_ context.Context, prefix string, limit int) ([]domaingeo.Province, error) {
	f.lastProvPrefix = prefix
	f.lastLimit = limit
	if f.searchProvErr != nil {
		return nil, f.searchProvErr
	}
	var out []domaingeo.Province
	for _, p := range f.provinces {
		out = append(out, p)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeGeoRepo) GetActiveProvinceID(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	if f.getProvErr != nil {
		return uuid.Nil, f.getProvErr
	}
	if f.provinceIDs != nil && !f.provinceIDs[id] {
		return uuid.Nil, apperr.NotFound("İl bulunamadı.")
	}
	return id, nil
}

func (f *fakeGeoRepo) GetActiveDistrict(_ context.Context, id uuid.UUID) (domaingeo.District, error) {
	for _, d := range f.districts {
		if d.ID == id && d.IsActive {
			return d, nil
		}
	}
	return domaingeo.District{}, apperr.NotFound("İlçe bulunamadı.")
}

func (f *fakeGeoRepo) ListActiveDistrictsByProvince(_ context.Context, provinceID uuid.UUID) ([]domaingeo.District, error) {
	if f.listDistErr != nil {
		return nil, f.listDistErr
	}
	var out []domaingeo.District
	for _, d := range f.districts {
		if d.ProvinceID == provinceID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeGeoRepo) SearchActiveDistrictsByNormalizedPrefix(_ context.Context, prefix string, provinceID *uuid.UUID, limit int) ([]domaingeo.District, error) {
	f.lastDistPrefix = prefix
	f.lastLimit = limit
	if f.searchDistErr != nil {
		return nil, f.searchDistErr
	}
	var out []domaingeo.District
	for _, d := range f.districts {
		if provinceID != nil && d.ProvinceID != *provinceID {
			continue
		}
		out = append(out, d)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func TestListActiveProvincesSuccessAndEmpty(t *testing.T) {
	id := uuid.New()
	svc := appgeo.NewService(&fakeGeoRepo{provinces: []domaingeo.Province{{ID: id, Name: "Ankara", SortOrder: 1}}})
	got, err := svc.ListActiveProvinces(context.Background())
	if err != nil || len(got) != 1 || got[0].Name != "Ankara" {
		t.Fatalf("got=%v err=%v", got, err)
	}

	svc = appgeo.NewService(&fakeGeoRepo{})
	got, err = svc.ListActiveProvinces(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("empty got=%v err=%v", got, err)
	}
}

func TestSearchProvincesNormalizesAndValidatesLimit(t *testing.T) {
	repo := &fakeGeoRepo{provinces: []domaingeo.Province{{ID: uuid.New(), Name: "İstanbul"}}}
	svc := appgeo.NewService(repo)
	q := "İST"
	limit := 5
	got, err := svc.SearchProvinces(context.Background(), &q, &limit)
	if err != nil || len(got) != 1 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if repo.lastProvPrefix != "ist" || repo.lastLimit != 5 {
		t.Fatalf("prefix=%q limit=%d", repo.lastProvPrefix, repo.lastLimit)
	}

	for _, bad := range []int{0, -1, 101} {
		bad := bad
		_, err = svc.SearchProvinces(context.Background(), &q, &bad)
		ae, ok := apperr.As(err)
		if !ok || ae.Code != apperr.CodeValidation {
			t.Fatalf("limit=%d want validation, got %v", bad, err)
		}
	}

	ws := "   "
	empty, err := svc.SearchProvinces(context.Background(), &ws, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("whitespace query got=%v err=%v", empty, err)
	}

	empty, err = svc.SearchProvinces(context.Background(), nil, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty query got=%v err=%v", empty, err)
	}
}

func TestListDistrictsNotFoundAndRepoError(t *testing.T) {
	missing := uuid.New()
	svc := appgeo.NewService(&fakeGeoRepo{provinceIDs: map[uuid.UUID]bool{}})
	_, err := svc.ListDistrictsByProvince(context.Background(), missing)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("want not found, got %v", err)
	}

	svc = appgeo.NewService(&fakeGeoRepo{listErr: errors.New("db down password=secret")})
	_, err = svc.ListActiveProvinces(context.Background())
	ae, ok = apperr.As(err)
	if !ok || ae.Kind != apperr.KindInternal {
		t.Fatalf("want internal, got %v", err)
	}
}

func TestSearchDistrictsScoped(t *testing.T) {
	pid := uuid.New()
	other := uuid.New()
	repo := &fakeGeoRepo{
		provinceIDs: map[uuid.UUID]bool{pid: true},
		districts: []domaingeo.District{
			{ID: uuid.New(), ProvinceID: pid, Name: "Çankaya"},
			{ID: uuid.New(), ProvinceID: other, Name: "Other"},
		},
	}
	svc := appgeo.NewService(repo)
	q := "can"
	got, err := svc.SearchDistricts(context.Background(), &q, &pid, nil)
	if err != nil || len(got) != 1 || got[0].Name != "Çankaya" {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if repo.lastDistPrefix != "can" || repo.lastLimit != 20 {
		t.Fatalf("prefix=%q limit=%d", repo.lastDistPrefix, repo.lastLimit)
	}
}

func TestListActiveProvincesUnavailableWhenSyncLeavesEmpty(t *testing.T) {
	src := &fakeSource{cat: domaingeo.Catalog{
		Provinces: []domaingeo.Province{{Name: "Ankara"}},
		Districts: []domaingeo.District{{Name: "Çankaya"}},
	}}
	store := &fakeStore{}
	svc := appgeo.NewService(&fakeGeoRepo{}).WithCatalogSync(appgeo.NewCatalogSync(src, store, time.Hour))
	_, err := svc.ListActiveProvinces(context.Background())
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("want unavailable, got %v", err)
	}
}
