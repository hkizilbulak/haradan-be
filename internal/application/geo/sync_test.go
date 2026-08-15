package geo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	appgeo "github.com/hkizilbulak/haradan-be/internal/application/geo"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
)

type fakeSource struct {
	cat   domaingeo.Catalog
	err   error
	calls int
}

func (f *fakeSource) FetchCatalog(context.Context) (domaingeo.Catalog, error) {
	f.calls++
	if f.err != nil {
		return domaingeo.Catalog{}, f.err
	}
	return f.cat, nil
}

type fakeStore struct {
	count      int
	countErr   error
	replace    int
	replaceErr error
	last       domaingeo.Catalog
}

func (f *fakeStore) CountActiveProvinces(context.Context) (int, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	return f.count, nil
}

func (f *fakeStore) ReplaceCatalog(_ context.Context, provinces []domaingeo.Province, districts []domaingeo.District) error {
	if f.replaceErr != nil {
		return f.replaceErr
	}
	f.replace++
	f.count = len(provinces)
	f.last = domaingeo.Catalog{Provinces: provinces, Districts: districts}
	return nil
}

func TestCatalogSyncFetchesWhenEmpty(t *testing.T) {
	src := &fakeSource{cat: domaingeo.Catalog{
		Provinces: []domaingeo.Province{{Name: "Ankara"}},
		Districts: []domaingeo.District{{Name: "Çankaya"}},
	}}
	store := &fakeStore{}
	sync := appgeo.NewCatalogSync(src, store, time.Hour)
	if err := sync.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if src.calls != 1 || store.replace != 1 || store.last.Provinces[0].Name != "Ankara" {
		t.Fatalf("calls=%d replace=%d cat=%+v", src.calls, store.replace, store.last)
	}
	if err := sync.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if src.calls != 1 {
		t.Fatalf("ttl should skip refetch, calls=%d", src.calls)
	}
}

func TestCatalogSyncServesStaleWhenLiveFails(t *testing.T) {
	src := &fakeSource{err: errors.New("upstream down")}
	store := &fakeStore{count: 81}
	sync := appgeo.NewCatalogSync(src, store, time.Hour)
	if err := sync.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.replace != 0 {
		t.Fatal("must not wipe local catalog on upstream failure")
	}
}

func TestCatalogSyncUnavailableWhenEmptyAndLiveFails(t *testing.T) {
	src := &fakeSource{err: errors.New("upstream down")}
	store := &fakeStore{}
	sync := appgeo.NewCatalogSync(src, store, time.Hour)
	err := sync.Ensure(context.Background())
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("want 503-class, got %v", err)
	}
}
