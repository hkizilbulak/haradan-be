package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository implements catalog.Repository with pgx.
type Repository struct {
	pool *pgxpool.Pool
	db   Querier
}

// NewRepository constructs a PostgreSQL catalog repository.
func NewRepository(db Querier) *Repository {
	pool, _ := db.(*pgxpool.Pool)
	return &Repository{pool: pool, db: db}
}

// WithTx returns a repository scoped to a transaction.
func (r *Repository) WithTx(tx pgx.Tx) *Repository { return &Repository{pool: r.pool, db: tx} }

// BeginTx starts a write transaction.
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	if r.pool == nil {
		return nil, apperr.Internal(errors.New("catalog repository has no pool"))
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("begin catalog tx: %w", pg.SanitizeErr(err)))
	}
	return tx, nil
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

const adminPropertyColumns = `id, category_id, code, title, help_text, data_type, is_required, is_filterable,
sort_order, options, validation, default_value, ui_metadata, is_active, is_form_visible, is_public_visible, version`

func (r *Repository) ListCategoriesAdmin(ctx context.Context, active *bool, limit int) ([]domaincatalog.Category, error) {
	q := `SELECT id, parent_id, slug, name, description, is_active, sort_order, version, created_at, updated_at FROM hrd_categories`
	args := []any{}
	if active != nil {
		q += ` WHERE is_active = $1`
		args = append(args, *active)
	}
	q += ` ORDER BY sort_order, name, id LIMIT $` + fmt.Sprint(len(args)+1)
	args = append(args, limit)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list admin categories: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := []domaincatalog.Category{}
	for rows.Next() {
		var c domaincatalog.Category
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Slug, &c.Name, &c.Description, &c.IsActive, &c.SortOrder, &c.Version, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, apperr.Internal(err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) GetCategoryAdmin(ctx context.Context, id uuid.UUID) (domaincatalog.Category, error) {
	var c domaincatalog.Category
	err := r.db.QueryRow(ctx, `SELECT id,parent_id,slug,name,description,is_active,sort_order,version,created_at,updated_at FROM hrd_categories WHERE id=$1`, id).Scan(&c.ID, &c.ParentID, &c.Slug, &c.Name, &c.Description, &c.IsActive, &c.SortOrder, &c.Version, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, apperr.NotFound("Kategori bulunamadı.")
	}
	if err != nil {
		return c, apperr.Internal(fmt.Errorf("get category: %w", pg.SanitizeErr(err)))
	}
	return c, nil
}

func (r *Repository) CreateCategory(ctx context.Context, c domaincatalog.Category) (domaincatalog.Category, error) {
	const q = `INSERT INTO hrd_categories(id,parent_id,slug,name,description,is_active,sort_order,version,created_at,updated_at) VALUES($1,$2,$3,$4,$5,true,$6,1,$7,$7) RETURNING id,parent_id,slug,name,description,is_active,sort_order,version,created_at,updated_at`
	var out domaincatalog.Category
	err := r.db.QueryRow(ctx, q, c.ID, c.ParentID, c.Slug, c.Name, c.Description, c.SortOrder, c.CreatedAt).Scan(&out.ID, &out.ParentID, &out.Slug, &out.Name, &out.Description, &out.IsActive, &out.SortOrder, &out.Version, &out.CreatedAt, &out.UpdatedAt)
	return out, r.writeCategoryErr(err, "create category")
}

func (r *Repository) UpdateCategory(ctx context.Context, id uuid.UUID, p domaincatalog.CategoryPatch, expected int, now time.Time) (domaincatalog.Category, error) {
	const q = `UPDATE hrd_categories SET slug=CASE WHEN $3 THEN $4 ELSE slug END,name=CASE WHEN $5 THEN $6 ELSE name END,description=CASE WHEN $7 THEN $8 ELSE description END,sort_order=CASE WHEN $9 THEN $10 ELSE sort_order END,version=version+1,updated_at=$11 WHERE id=$1 AND version=$2 RETURNING id,parent_id,slug,name,description,is_active,sort_order,version,created_at,updated_at`
	var out domaincatalog.Category
	err := r.db.QueryRow(ctx, q, id, expected, p.SlugSet, p.Slug, p.NameSet, p.Name, p.DescriptionSet, p.Description, p.SortOrderSet, p.SortOrder, now).Scan(&out.ID, &out.ParentID, &out.Slug, &out.Name, &out.Description, &out.IsActive, &out.SortOrder, &out.Version, &out.CreatedAt, &out.UpdatedAt)
	return out, r.writeCategoryErr(err, "update category")
}

func (r *Repository) SetCategoryActive(ctx context.Context, id uuid.UUID, active bool, expected int, now time.Time) (domaincatalog.Category, error) {
	const q = `UPDATE hrd_categories SET is_active=$3,version=version+1,updated_at=$4 WHERE id=$1 AND version=$2 RETURNING id,parent_id,slug,name,description,is_active,sort_order,version,created_at,updated_at`
	var out domaincatalog.Category
	err := r.db.QueryRow(ctx, q, id, expected, active, now).Scan(&out.ID, &out.ParentID, &out.Slug, &out.Name, &out.Description, &out.IsActive, &out.SortOrder, &out.Version, &out.CreatedAt, &out.UpdatedAt)
	return out, r.writeCategoryErr(err, "set category active")
}

func (r *Repository) ReparentCategory(ctx context.Context, id uuid.UUID, parent *uuid.UUID, expected int, now time.Time) (domaincatalog.Category, error) {
	const q = `UPDATE hrd_categories SET parent_id=$3,version=version+1,updated_at=$4 WHERE id=$1 AND version=$2 RETURNING id,parent_id,slug,name,description,is_active,sort_order,version,created_at,updated_at`
	var out domaincatalog.Category
	err := r.db.QueryRow(ctx, q, id, expected, parent, now).Scan(&out.ID, &out.ParentID, &out.Slug, &out.Name, &out.Description, &out.IsActive, &out.SortOrder, &out.Version, &out.CreatedAt, &out.UpdatedAt)
	return out, r.writeCategoryErr(err, "reparent category")
}

func (r *Repository) IsDescendant(ctx context.Context, child, parent uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `WITH RECURSIVE ancestors AS (
SELECT id,parent_id,ARRAY[id] AS path FROM hrd_categories WHERE id=$1
UNION ALL
SELECT c.id,c.parent_id,a.path || c.id FROM hrd_categories c JOIN ancestors a ON a.parent_id=c.id
WHERE NOT c.id = ANY(a.path)
) SELECT EXISTS(SELECT 1 FROM ancestors WHERE id=$2)`, parent, child).Scan(&exists)
	return exists, err
}

func (r *Repository) ReorderCategories(ctx context.Context, items []domaincatalog.ReorderItem, now time.Time) error {
	return r.transactionalReorder(ctx, `hrd_categories`, `id`, items, now)
}

func (r *Repository) ListPropertiesAdmin(ctx context.Context, categoryID uuid.UUID) ([]domaincatalog.Property, error) {
	rows, err := r.db.Query(ctx, `SELECT `+adminPropertyColumns+` FROM hrd_category_properties WHERE category_id=$1 ORDER BY sort_order,code,id`, categoryID)
	if err != nil {
		return nil, apperr.Internal(pg.SanitizeErr(err))
	}
	defer rows.Close()
	out := []domaincatalog.Property{}
	for rows.Next() {
		var p domaincatalog.Property
		if err := scanAdminProperty(rows, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (r *Repository) CreateProperty(ctx context.Context, p domaincatalog.Property, now time.Time) (domaincatalog.Property, error) {
	const q = `INSERT INTO hrd_category_properties(id,category_id,code,title,help_text,data_type,is_required,is_public_visible,is_form_visible,is_filterable,sort_order,is_active,options,validation,default_value,ui_metadata,version,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,true,$12::jsonb,$13::jsonb,$14::jsonb,$15::jsonb,1,$16,$16) RETURNING ` + adminPropertyColumns
	var out domaincatalog.Property
	err := scanAdminProperty(r.db.QueryRow(ctx, q, p.ID, p.CategoryID, p.Code, p.Title, p.HelpText, p.DataType, p.IsRequired, p.IsPublicVisible, p.IsFormVisible, p.IsFilterable, p.SortOrder, p.Options, p.Validation, p.DefaultValue, p.UIMetadata, now), &out)
	return out, r.writeCategoryErr(err, "create property")
}
func (r *Repository) UpdateProperty(ctx context.Context, id, categoryID uuid.UUID, p domaincatalog.PropertyPatch, expected int, now time.Time) (domaincatalog.Property, error) {
	const q = `UPDATE hrd_category_properties SET title=CASE WHEN $4 THEN $5 ELSE title END,help_text=CASE WHEN $6 THEN $7 ELSE help_text END,is_required=CASE WHEN $8 THEN $9 ELSE is_required END,is_public_visible=CASE WHEN $10 THEN $11 ELSE is_public_visible END,is_form_visible=CASE WHEN $12 THEN $13 ELSE is_form_visible END,is_filterable=CASE WHEN $14 THEN $15 ELSE is_filterable END,sort_order=CASE WHEN $16 THEN $17 ELSE sort_order END,options=CASE WHEN $18 THEN $19::jsonb ELSE options END,validation=CASE WHEN $20 THEN $21::jsonb ELSE validation END,default_value=CASE WHEN $22 THEN $23::jsonb ELSE default_value END,ui_metadata=CASE WHEN $24 THEN $25::jsonb ELSE ui_metadata END,version=version+1,updated_at=$26 WHERE id=$1 AND category_id=$2 AND version=$3 RETURNING ` + adminPropertyColumns
	var out domaincatalog.Property
	err := scanAdminProperty(r.db.QueryRow(ctx, q, id, categoryID, expected, p.TitleSet, p.Title, p.HelpTextSet, p.HelpText, p.IsRequiredSet, p.IsRequired, p.IsPublicVisibleSet, p.IsPublicVisible, p.IsFormVisibleSet, p.IsFormVisible, p.IsFilterableSet, p.IsFilterable, p.SortOrderSet, p.SortOrder, p.OptionsSet, p.Options, p.ValidationSet, p.Validation, p.DefaultValueSet, p.DefaultValue, p.UIMetadataSet, p.UIMetadata, now), &out)
	return out, r.writeCategoryErr(err, "update property")
}
func (r *Repository) SetPropertyActive(ctx context.Context, id, categoryID uuid.UUID, active bool, expected int, now time.Time) (domaincatalog.Property, error) {
	const q = `UPDATE hrd_category_properties SET is_active=$4,version=version+1,updated_at=$5 WHERE id=$1 AND category_id=$2 AND version=$3 RETURNING ` + adminPropertyColumns
	var out domaincatalog.Property
	err := scanAdminProperty(r.db.QueryRow(ctx, q, id, categoryID, expected, active, now), &out)
	return out, r.writeCategoryErr(err, "set property active")
}
func (r *Repository) ReorderProperties(ctx context.Context, items []domaincatalog.ReorderItem, now time.Time) error {
	return r.transactionalReorder(ctx, `hrd_category_properties`, `id`, items, now)
}
func (r *Repository) transactionalReorder(ctx context.Context, table, key string, items []domaincatalog.ReorderItem, now time.Time) (err error) {
	if r.pool == nil {
		return r.reorder(ctx, table, key, items, now)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(fmt.Errorf("begin catalog reorder: %w", pg.SanitizeErr(err)))
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = r.WithTx(tx).reorder(ctx, table, key, items, now); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return apperr.Internal(fmt.Errorf("commit catalog reorder: %w", pg.SanitizeErr(err)))
	}
	return nil
}
func (r *Repository) reorder(ctx context.Context, table, key string, items []domaincatalog.ReorderItem, now time.Time) error {
	for _, i := range items {
		tag, err := r.db.Exec(ctx, `UPDATE `+table+` SET sort_order=$3,version=version+1,updated_at=$4 WHERE `+key+`=$1 AND version=$2`, i.ID, i.ExpectedVersion, i.SortOrder, now)
		if err != nil {
			return apperr.Internal(pg.SanitizeErr(err))
		}
		if tag.RowsAffected() != 1 {
			return apperr.StaleVersion("Kayıt başka bir işlem tarafından güncellendi.")
		}
	}
	return nil
}
func scanAdminProperty(row interface{ Scan(...any) error }, p *domaincatalog.Property) error {
	return row.Scan(&p.ID, &p.CategoryID, &p.Code, &p.Title, &p.HelpText, &p.DataType, &p.IsRequired, &p.IsFilterable, &p.SortOrder, &p.Options, &p.Validation, &p.DefaultValue, &p.UIMetadata, &p.IsActive, &p.IsFormVisible, &p.IsPublicVisible, &p.Version)
}
func (r *Repository) writeCategoryErr(err error, op string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.StaleVersion("Kayıt başka bir işlem tarafından güncellendi.")
	}
	if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
		return apperr.Conflict("Kayıt zaten mevcut.")
	}
	return apperr.Internal(fmt.Errorf("%s: %w", op, pg.SanitizeErr(err)))
}

// HardDeleteCategory permanently deletes a category and its properties from the database.
func (r *Repository) HardDeleteCategory(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM hrd_category_properties WHERE category_id = $1`, id); err != nil {
		return r.writeCategoryErr(err, "delete category properties")
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM hrd_categories WHERE id = $1`, id)
	if err != nil {
		return r.writeCategoryErr(err, "delete category")
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("Kategori bulunamadı.")
	}
	return nil
}

// HardDeleteCategoryProperty permanently deletes a single category property from the database.
func (r *Repository) HardDeleteCategoryProperty(ctx context.Context, categoryID, propID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM hrd_category_properties WHERE id = $1 AND category_id = $2`, propID, categoryID)
	if err != nil {
		return r.writeCategoryErr(err, "delete category property")
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("Kategori özelliği bulunamadı.")
	}
	return nil
}

