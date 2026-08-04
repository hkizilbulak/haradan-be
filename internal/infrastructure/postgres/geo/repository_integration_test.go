package geo_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

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
