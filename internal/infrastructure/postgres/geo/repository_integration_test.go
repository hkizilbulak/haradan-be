package geo_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
	pggeo "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/geo"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/testutil"
)

func TestRepositoryListActiveProvincesIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	id := uuid.New()
	now := time.Now().UTC()
	_, err := tx.Exec(ctx, `
INSERT INTO hrd_provinces (id, name, name_normalized, is_active, sort_order, created_at, updated_at)
VALUES ($1, 'Ankara', 'ankara', true, 1, $2, $2)`, id, now)
	if err != nil {
		t.Fatalf("insert province: %v", err)
	}

	repo := pggeo.NewRepository(tx)
	got, err := repo.ListActiveProvinces(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range got {
		if p.ID == id {
			found = true
			if p.Name != "Ankara" {
				t.Fatalf("name=%q", p.Name)
			}
		}
	}
	if !found {
		t.Fatalf("inserted province not returned: %+v", got)
	}
}

func TestRepositorySearchProvincesPrefixIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	id := uuid.New()
	now := time.Now().UTC()
	_, err := tx.Exec(ctx, `
INSERT INTO hrd_provinces (id, name, name_normalized, is_active, sort_order, created_at, updated_at)
VALUES ($1, 'İstanbul', 'istanbul', true, 1, $2, $2)`, id, now)
	if err != nil {
		t.Fatalf("insert province: %v", err)
	}

	repo := pggeo.NewRepository(tx)
	got, err := repo.SearchActiveProvincesByNormalizedPrefix(ctx, "ist", 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range got {
		if p.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("prefix search missed istanbul: %+v", got)
	}
}

func TestRepositoryReplaceCatalogIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	pID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	dID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := pggeo.NewRepository(tx)
	err := repo.ReplaceCatalog(ctx, []domaingeo.Province{{
		ID: pID, Name: "HaradanTestİl", SortOrder: 6, IsActive: true,
	}}, []domaingeo.District{{
		ID: dID, ProvinceID: pID, Name: "HaradanTestİlçe", SortOrder: 1, IsActive: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	n, err := repo.CountActiveProvinces(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("count=%d", n)
	}
	got, err := repo.ListActiveDistrictsByProvince(ctx, pID)
	if err != nil || len(got) != 1 || got[0].Name != "HaradanTestİlçe" {
		t.Fatalf("districts=%v err=%v", got, err)
	}
}
