// Package banner persists hrd_banners.
package banner

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainbanner "github.com/hkizilbulak/haradan-be/internal/domain/banner"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

const columns = `id, placement, status, asset_id, title, alt_text, target_url, sort_order, version, created_by_user_id, created_at, updated_at`

type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Repository struct {
	pool *pgxpool.Pool
	db   Querier
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool, db: pool} }
func (r *Repository) WithTx(tx pgx.Tx) *Repository { return &Repository{pool: r.pool, db: tx} }
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	if r.pool == nil {
		return nil, apperr.Internal(errors.New("banner repository has no pool"))
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("begin banner tx: %w", pg.SanitizeErr(err)))
	}
	return tx, nil
}
func (r *Repository) Create(ctx context.Context, b domainbanner.Banner) error {
	_, err := r.db.Exec(ctx, `INSERT INTO hrd_banners (id,placement,status,asset_id,title,alt_text,target_url,sort_order,version,created_by_user_id,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, b.ID, string(b.Placement), string(b.Status), b.AssetID, b.Title, b.AltText, b.TargetURL, b.SortOrder, b.Version, b.CreatedByUserID, b.CreatedAt, b.UpdatedAt)
	return writeErr("create banner", err)
}
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (domainbanner.Banner, error) {
	return r.one(ctx, "get banner", `SELECT `+columns+` FROM hrd_banners WHERE id=$1`, id)
}
func (r *Repository) LockByID(ctx context.Context, id uuid.UUID) (domainbanner.Banner, error) {
	return r.one(ctx, "lock banner", `SELECT `+columns+` FROM hrd_banners WHERE id=$1 FOR UPDATE`, id)
}
func (r *Repository) one(ctx context.Context, op, q string, args ...any) (domainbanner.Banner, error) {
	b, err := scan(r.db.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainbanner.Banner{}, apperr.NotFound("Banner bulunamadı.")
	}
	if err != nil {
		return domainbanner.Banner{}, apperr.Internal(fmt.Errorf("%s: %w", op, pg.SanitizeErr(err)))
	}
	return b, nil
}

type ListFilter struct {
	Placement *domainbanner.Placement
	Status    *domainbanner.Status
	Limit     int
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]domainbanner.Banner, error) {
	var q strings.Builder
	args := []any{}
	q.WriteString(`SELECT ` + columns + ` FROM hrd_banners WHERE 1=1`)
	if f.Placement != nil {
		args = append(args, string(*f.Placement))
		fmt.Fprintf(&q, ` AND placement=$%d`, len(args))
	}
	if f.Status != nil {
		args = append(args, string(*f.Status))
		fmt.Fprintf(&q, ` AND status=$%d`, len(args))
	}
	q.WriteString(` ORDER BY sort_order ASC,id ASC`)
	if f.Limit > 0 {
		args = append(args, f.Limit)
		fmt.Fprintf(&q, ` LIMIT $%d`, len(args))
	}
	return r.many(ctx, "list banners", q.String(), args...)
}
func (r *Repository) ListActive(ctx context.Context, p domainbanner.Placement) ([]domainbanner.Banner, error) {
	return r.many(ctx, "list active banners", `SELECT `+columns+` FROM hrd_banners WHERE placement=$1 AND status='ACTIVE' ORDER BY sort_order ASC,id ASC`, string(p))
}
func (r *Repository) many(ctx context.Context, op, q string, args ...any) ([]domainbanner.Banner, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("%s: %w", op, pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := []domainbanner.Banner{}
	for rows.Next() {
		b, e := scan(rows)
		if e != nil {
			return nil, apperr.Internal(fmt.Errorf("scan banner: %w", e))
		}
		out = append(out, b)
	}
	if err = rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("%s: %w", op, pg.SanitizeErr(err)))
	}
	return out, nil
}
func (r *Repository) UpdateOptimistic(ctx context.Context, b domainbanner.Banner, expected int) (domainbanner.Banner, error) {
	q := `UPDATE hrd_banners SET placement=$3,status=$4,asset_id=$5,title=$6,alt_text=$7,target_url=$8,sort_order=$9,version=version+1,updated_at=$10 WHERE id=$1 AND version=$2 RETURNING ` + columns
	out, err := scan(r.db.QueryRow(ctx, q, b.ID, expected, string(b.Placement), string(b.Status), b.AssetID, b.Title, b.AltText, b.TargetURL, b.SortOrder, b.UpdatedAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainbanner.Banner{}, apperr.StaleVersion("Banner başka bir işlem tarafından güncellendi.")
	}
	if err != nil {
		return domainbanner.Banner{}, writeErr("update banner", err)
	}
	return out, nil
}

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domainbanner.Banner, error) {
	var b domainbanner.Banner
	var p, s string
	err := row.Scan(&b.ID, &p, &s, &b.AssetID, &b.Title, &b.AltText, &b.TargetURL, &b.SortOrder, &b.Version, &b.CreatedByUserID, &b.CreatedAt, &b.UpdatedAt)
	b.Placement = domainbanner.Placement(p)
	b.Status = domainbanner.Status(s)
	return b, err
}
func writeErr(op string, err error) error {
	if err == nil {
		return nil
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && (pe.Code == "23514" || pe.Code == "23503") {
		return apperr.Validation("Banner geçersiz.")
	}
	return apperr.Internal(fmt.Errorf("%s: %w", op, pg.SanitizeErr(err)))
}
