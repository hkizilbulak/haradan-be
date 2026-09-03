package catalog_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	pgcatalog "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/catalog"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/testutil"
)

func TestRepositoryListFormPropertiesOrderedIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	catID := uuid.New()
	now := time.Now().UTC()
	_, err := tx.Exec(ctx, `
INSERT INTO hrd_categories (id, parent_id, slug, name, description, is_active, sort_order, version, created_at, updated_at)
VALUES ($1, NULL, $2, 'Leaf', NULL, true, 1, 1, $3, $3)`, catID, "leaf-"+catID.String()[:8], now)
	if err != nil {
		t.Fatalf("insert category: %v", err)
	}

	p1, p2 := uuid.New(), uuid.New()
	_, err = tx.Exec(ctx, `
INSERT INTO hrd_category_properties (
  id, category_id, code, title, help_text, data_type, is_required, is_public_visible, is_form_visible,
  is_filterable, sort_order, is_active, options, validation, default_value, ui_metadata, version, created_at, updated_at
) VALUES
($1, $3, 'b', 'B', NULL, 'STRING', false, true, true, false, 2, true, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb, 1, $4, $4),
($2, $3, 'a', 'A', NULL, 'STRING', false, true, true, false, 1, true, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb, 1, $4, $4)
`, p1, p2, catID, now)
	if err != nil {
		t.Fatalf("insert properties: %v", err)
	}

	repo := pgcatalog.NewRepository(tx)
	got, err := repo.ListFormProperties(ctx, catID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Code != "a" || got[1].Code != "b" {
		t.Fatalf("order=%+v", got)
	}
}

func TestListStudFormPropertiesIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	asimID := uuid.MustParse("c1000000-0000-4000-8000-000000000003")
	arapID := uuid.MustParse("c1000000-0000-4000-8000-000000000031")
	ingilizID := uuid.MustParse("c1000000-0000-4000-8000-000000000032")

	repo := pgcatalog.NewRepository(tx)
	for _, id := range []uuid.UUID{asimID, arapID, ingilizID} {
		props, err := repo.ListFormProperties(ctx, id)
		if err != nil {
			t.Fatalf("list properties for %s: %v", id, err)
		}
		t.Logf("=== Properties for %s (count: %d) ===", id, len(props))
		for _, p := range props {
			t.Logf(" - Code: %s, Title: %s, DataType: %s, IsRequired: %v, Options: %s", p.Code, p.Title, p.DataType, p.IsRequired, string(p.Options))
		}
	}
}

