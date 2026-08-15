package geo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
	"github.com/hkizilbulak/haradan-be/internal/platform/textnorm"
)

type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// CountActiveProvinces returns how many active provinces are stored locally.
func (r *Repository) CountActiveProvinces(ctx context.Context) (int, error) {
	const q = `SELECT COUNT(*) FROM hrd_provinces WHERE is_active = true`
	var n int
	if err := r.db.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("count provinces: %w", pg.SanitizeErr(err))
	}
	return n, nil
}

// ReplaceCatalog upserts the live catalog and deactivates rows that disappeared.
func (r *Repository) ReplaceCatalog(ctx context.Context, provinces []domaingeo.Province, districts []domaingeo.District) error {
	if b, ok := r.db.(txBeginner); ok {
		tx, err := b.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin geo catalog tx: %w", pg.SanitizeErr(err))
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if err := replaceCatalog(ctx, tx, provinces, districts); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit geo catalog: %w", pg.SanitizeErr(err))
		}
		return nil
	}
	return replaceCatalog(ctx, r.db, provinces, districts)
}

func replaceCatalog(ctx context.Context, db Querier, provinces []domaingeo.Province, districts []domaingeo.District) error {
	if len(provinces) == 0 || len(districts) == 0 {
		return fmt.Errorf("geo catalog snapshot is empty")
	}
	now := time.Now().UTC()
	provinceIDs := make([]uuid.UUID, len(provinces))
	provinceNames := make([]string, len(provinces))
	provinceNorms := make([]string, len(provinces))
	provinceOrder := make([]int32, len(provinces))
	for i, p := range provinces {
		provinceIDs[i] = p.ID
		provinceNames[i] = p.Name
		provinceNorms[i] = textnorm.TurkishFold(p.Name)
		provinceOrder[i] = int32(p.SortOrder)
	}
	if _, err := db.Exec(ctx, `
INSERT INTO hrd_provinces (id, name, name_normalized, is_active, sort_order, created_at, updated_at)
SELECT t.id, t.name, t.name_normalized, true, t.sort_order, $5, $5
FROM unnest($1::uuid[], $2::text[], $3::text[], $4::int[]) AS t(id, name, name_normalized, sort_order)
ON CONFLICT (id) DO UPDATE SET
	name = EXCLUDED.name,
	name_normalized = EXCLUDED.name_normalized,
	is_active = true,
	sort_order = EXCLUDED.sort_order,
	updated_at = EXCLUDED.updated_at`,
		provinceIDs, provinceNames, provinceNorms, provinceOrder, now); err != nil {
		return fmt.Errorf("upsert provinces: %w", pg.SanitizeErr(err))
	}

	districtIDs := make([]uuid.UUID, len(districts))
	districtProvinceIDs := make([]uuid.UUID, len(districts))
	districtNames := make([]string, len(districts))
	districtNorms := make([]string, len(districts))
	districtOrder := make([]int32, len(districts))
	for i, d := range districts {
		districtIDs[i] = d.ID
		districtProvinceIDs[i] = d.ProvinceID
		districtNames[i] = d.Name
		districtNorms[i] = textnorm.TurkishFold(d.Name)
		districtOrder[i] = int32(d.SortOrder)
	}
	if _, err := db.Exec(ctx, `
INSERT INTO hrd_districts (id, province_id, name, name_normalized, is_active, sort_order, created_at, updated_at)
SELECT t.id, t.province_id, t.name, t.name_normalized, true, t.sort_order, $6, $6
FROM unnest($1::uuid[], $2::uuid[], $3::text[], $4::text[], $5::int[]) AS t(id, province_id, name, name_normalized, sort_order)
ON CONFLICT (id) DO UPDATE SET
	province_id = EXCLUDED.province_id,
	name = EXCLUDED.name,
	name_normalized = EXCLUDED.name_normalized,
	is_active = true,
	sort_order = EXCLUDED.sort_order,
	updated_at = EXCLUDED.updated_at`,
		districtIDs, districtProvinceIDs, districtNames, districtNorms, districtOrder, now); err != nil {
		return fmt.Errorf("upsert districts: %w", pg.SanitizeErr(err))
	}
	if _, err := db.Exec(ctx, `
UPDATE hrd_provinces
SET is_active = false, updated_at = $2
WHERE is_active = true AND NOT (id = ANY($1::uuid[]))`, provinceIDs, now); err != nil {
		return fmt.Errorf("deactivate provinces: %w", pg.SanitizeErr(err))
	}
	if _, err := db.Exec(ctx, `
UPDATE hrd_districts
SET is_active = false, updated_at = $2
WHERE is_active = true AND NOT (id = ANY($1::uuid[]))`, districtIDs, now); err != nil {
		return fmt.Errorf("deactivate districts: %w", pg.SanitizeErr(err))
	}
	return nil
}
