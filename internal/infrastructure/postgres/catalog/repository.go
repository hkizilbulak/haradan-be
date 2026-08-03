package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository implements catalog.Repository with pgx.
type Repository struct {
	db Querier
}

// NewRepository constructs a PostgreSQL catalog repository.
func NewRepository(db Querier) *Repository {
	return &Repository{db: db}
}

// ListActiveCategories returns all active categories.
func (r *Repository) ListActiveCategories(ctx context.Context) ([]domaincatalog.Category, error) {
	const q = `
SELECT id, parent_id, slug, name, description, is_active, sort_order, version, created_at, updated_at
FROM hrd_categories
WHERE is_active = true
ORDER BY sort_order ASC, name ASC, id ASC`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", pg.SanitizeErr(err))
	}
	defer rows.Close()

	var out []domaincatalog.Category
	for rows.Next() {
		var c domaincatalog.Category
		if err := rows.Scan(
			&c.ID, &c.ParentID, &c.Slug, &c.Name, &c.Description,
			&c.IsActive, &c.SortOrder, &c.Version, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan category: %w", pg.SanitizeErr(err))
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categories: %w", pg.SanitizeErr(err))
	}
	return out, nil
}

// GetActiveCategory returns an active category by id.
func (r *Repository) GetActiveCategory(ctx context.Context, id uuid.UUID) (domaincatalog.Category, error) {
	const q = `
SELECT id, parent_id, slug, name, description, is_active, sort_order, version, created_at, updated_at
FROM hrd_categories
WHERE id = $1 AND is_active = true`

	var c domaincatalog.Category
	err := r.db.QueryRow(ctx, q, id).Scan(
		&c.ID, &c.ParentID, &c.Slug, &c.Name, &c.Description,
		&c.IsActive, &c.SortOrder, &c.Version, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domaincatalog.Category{}, apperr.NotFound("Kategori bulunamadı.")
		}
		return domaincatalog.Category{}, fmt.Errorf("get category: %w", pg.SanitizeErr(err))
	}
	return c, nil
}

// CountActiveChildren counts active child categories.
func (r *Repository) CountActiveChildren(ctx context.Context, parentID uuid.UUID) (int, error) {
	const q = `
SELECT COUNT(*)
FROM hrd_categories
WHERE parent_id = $1 AND is_active = true`

	var n int
	if err := r.db.QueryRow(ctx, q, parentID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count category children: %w", pg.SanitizeErr(err))
	}
	return n, nil
}

// ListFormProperties returns active form-visible properties ordered by sort_order.
func (r *Repository) ListFormProperties(ctx context.Context, categoryID uuid.UUID) ([]domaincatalog.Property, error) {
	const q = `
SELECT id, category_id, code, title, help_text, data_type, is_required, is_filterable,
       sort_order, options, default_value, ui_metadata, is_active, is_form_visible, is_public_visible
FROM hrd_category_properties
WHERE category_id = $1
  AND is_active = true
  AND is_form_visible = true
ORDER BY sort_order ASC, code ASC, id ASC`

	rows, err := r.db.Query(ctx, q, categoryID)
	if err != nil {
		return nil, fmt.Errorf("list category properties: %w", pg.SanitizeErr(err))
	}
	defer rows.Close()

	var out []domaincatalog.Property
	for rows.Next() {
		var p domaincatalog.Property
		if err := rows.Scan(
			&p.ID, &p.CategoryID, &p.Code, &p.Title, &p.HelpText, &p.DataType,
			&p.IsRequired, &p.IsFilterable, &p.SortOrder, &p.Options, &p.DefaultValue,
			&p.UIMetadata, &p.IsActive, &p.IsFormVisible, &p.IsPublicVisible,
		); err != nil {
			return nil, fmt.Errorf("scan category property: %w", pg.SanitizeErr(err))
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate category properties: %w", pg.SanitizeErr(err))
	}
	return out, nil
}
