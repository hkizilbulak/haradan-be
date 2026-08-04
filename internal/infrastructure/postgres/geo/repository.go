package geo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository implements geo.Repository with pgx.
type Repository struct {
	db Querier
}

// NewRepository constructs a PostgreSQL geo repository.
func NewRepository(db Querier) *Repository {
	return &Repository{db: db}
}

// ListActiveProvinces returns active provinces ordered by sort_order, name, id.
func (r *Repository) ListActiveProvinces(ctx context.Context) ([]domaingeo.Province, error) {
	const q = `
SELECT id, name, sort_order, is_active, created_at, updated_at
FROM hrd_provinces
WHERE is_active = true
ORDER BY sort_order ASC, name ASC, id ASC`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list provinces: %w", pg.SanitizeErr(err))
	}
	defer rows.Close()

	var out []domaingeo.Province
	for rows.Next() {
		var p domaingeo.Province
		if err := rows.Scan(&p.ID, &p.Name, &p.SortOrder, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan province: %w", pg.SanitizeErr(err))
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provinces: %w", pg.SanitizeErr(err))
	}
	return out, nil
}

// SearchActiveProvincesByNormalizedPrefix searches active provinces by prefix.
func (r *Repository) SearchActiveProvincesByNormalizedPrefix(ctx context.Context, prefix string, limit int) ([]domaingeo.Province, error) {
	const q = `
SELECT id, name, sort_order, is_active, created_at, updated_at
FROM hrd_provinces
WHERE is_active = true
  AND name_normalized LIKE $1 || '%'
ORDER BY sort_order ASC, name ASC, id ASC
LIMIT $2`

	rows, err := r.db.Query(ctx, q, prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("search provinces: %w", pg.SanitizeErr(err))
	}
	defer rows.Close()

	var out []domaingeo.Province
	for rows.Next() {
		var p domaingeo.Province
		if err := rows.Scan(&p.ID, &p.Name, &p.SortOrder, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan province: %w", pg.SanitizeErr(err))
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provinces: %w", pg.SanitizeErr(err))
	}
	return out, nil
}

// GetActiveProvinceID returns the id when the province exists and is active.
func (r *Repository) GetActiveProvinceID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	const q = `
SELECT id
FROM hrd_provinces
WHERE id = $1 AND is_active = true`

	var found uuid.UUID
	err := r.db.QueryRow(ctx, q, id).Scan(&found)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, apperr.NotFound("İl bulunamadı.")
		}
		return uuid.Nil, fmt.Errorf("get province: %w", pg.SanitizeErr(err))
	}
	return found, nil
}

// GetActiveDistrict returns an active district by id.
func (r *Repository) GetActiveDistrict(ctx context.Context, id uuid.UUID) (domaingeo.District, error) {
	const q = `
SELECT id, province_id, name, sort_order, is_active, created_at, updated_at
FROM hrd_districts
WHERE id = $1 AND is_active = true`

	var d domaingeo.District
	err := r.db.QueryRow(ctx, q, id).Scan(
		&d.ID, &d.ProvinceID, &d.Name, &d.SortOrder, &d.IsActive, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domaingeo.District{}, apperr.NotFound("İlçe bulunamadı.")
		}
		return domaingeo.District{}, fmt.Errorf("get district: %w", pg.SanitizeErr(err))
	}
	return d, nil
}

// ListActiveDistrictsByProvince returns active districts for a province.
func (r *Repository) ListActiveDistrictsByProvince(ctx context.Context, provinceID uuid.UUID) ([]domaingeo.District, error) {
	const q = `
SELECT id, province_id, name, sort_order, is_active, created_at, updated_at
FROM hrd_districts
WHERE province_id = $1 AND is_active = true
ORDER BY sort_order ASC, name ASC, id ASC`

	rows, err := r.db.Query(ctx, q, provinceID)
	if err != nil {
		return nil, fmt.Errorf("list districts: %w", pg.SanitizeErr(err))
	}
	defer rows.Close()

	var out []domaingeo.District
	for rows.Next() {
		var d domaingeo.District
		if err := rows.Scan(&d.ID, &d.ProvinceID, &d.Name, &d.SortOrder, &d.IsActive, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan district: %w", pg.SanitizeErr(err))
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate districts: %w", pg.SanitizeErr(err))
	}
	return out, nil
}

// SearchActiveDistrictsByNormalizedPrefix searches active districts by prefix.
func (r *Repository) SearchActiveDistrictsByNormalizedPrefix(ctx context.Context, prefix string, provinceID *uuid.UUID, limit int) ([]domaingeo.District, error) {
	const q = `
SELECT id, province_id, name, sort_order, is_active, created_at, updated_at
FROM hrd_districts
WHERE is_active = true
  AND name_normalized LIKE $1 || '%'
  AND ($2::uuid IS NULL OR province_id = $2)
ORDER BY sort_order ASC, name ASC, id ASC
LIMIT $3`

	rows, err := r.db.Query(ctx, q, prefix, provinceID, limit)
	if err != nil {
		return nil, fmt.Errorf("search districts: %w", pg.SanitizeErr(err))
	}
	defer rows.Close()

	var out []domaingeo.District
	for rows.Next() {
		var d domaingeo.District
		if err := rows.Scan(&d.ID, &d.ProvinceID, &d.Name, &d.SortOrder, &d.IsActive, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan district: %w", pg.SanitizeErr(err))
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate districts: %w", pg.SanitizeErr(err))
	}
	return out, nil
}
