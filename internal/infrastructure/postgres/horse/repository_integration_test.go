package horse_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	pghorse "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/horse"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/testutil"
)

func TestHorseRepositoryFindAndSearchIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	id := uuid.New()
	now := time.Now().UTC()
	_, err := tx.Exec(ctx, `
INSERT INTO hrd_horses (
  id, tjk_number, original_name, name_normalized, birth_year, sire_name, dam_name,
  breed, gender, coat, detail, last_synced_at, created_at, updated_at
) VALUES (
  $1, 'TJK-42', 'İstanbul', 'istanbul', 2019, 'Sire', 'Dam',
  'İngiliz', 'Erkek', 'Doru', '{"pedigree":{}}'::jsonb, $2, $2, $2
)`, id, now)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	repo := pghorse.NewRepository(tx)

	got, err := repo.FindByID(ctx, id)
	if err != nil || got.OriginalName != "İstanbul" || got.TJKNumber != "TJK-42" {
		t.Fatalf("%+v err=%v", got, err)
	}

	byTJK, err := repo.FindByTJKNumber(ctx, "TJK-42")
	if err != nil || byTJK.ID != id {
		t.Fatalf("%+v err=%v", byTJK, err)
	}

	_, err = repo.FindByTJKNumber(ctx, "missing")
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeNotFound {
		t.Fatalf("err=%v", err)
	}

	found, err := repo.SearchByNormalizedPrefix(ctx, "ist", 10)
	if err != nil || len(found) != 1 || found[0].ID != id {
		t.Fatalf("%+v err=%v", found, err)
	}

	// Unique TJK race: second insert with same tjk must fail at DB level.
	_, err = tx.Exec(ctx, `
INSERT INTO hrd_horses (
  id, tjk_number, original_name, name_normalized, detail, created_at, updated_at
) VALUES ($1, 'TJK-42', 'Other', 'other', '{}'::jsonb, $2, $2)`, uuid.New(), now)
	if err == nil {
		t.Fatal("expected unique violation")
	}

	// Ensure detail round-trips as object JSON.
	var detail map[string]any
	if err := json.Unmarshal(got.Detail, &detail); err != nil {
		t.Fatal(err)
	}
	if _, ok := detail["pedigree"]; !ok {
		t.Fatalf("detail=%v", detail)
	}
}
