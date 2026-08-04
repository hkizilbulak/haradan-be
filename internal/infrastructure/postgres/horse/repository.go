package horse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainhorse "github.com/hkizilbulak/haradan-be/internal/domain/horse"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

const horseColumns = `id, tjk_number, original_name, name_normalized, birth_year, sire_name, dam_name,
breed, gender, coat, detail, last_synced_at, last_seen_at, source_updated_at, created_at, updated_at`

// Repository implements domainhorse.Repository with pgx.
type Repository struct {
	db Querier
}

// NewRepository constructs a PostgreSQL horse repository.
func NewRepository(db Querier) *Repository {
	return &Repository{db: db}
}

// FindByID returns a horse by primary key.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (domainhorse.Horse, error) {
	const q = `SELECT ` + horseColumns + ` FROM hrd_horses WHERE id = $1`
	h, err := scanHorse(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainhorse.Horse{}, apperr.NotFound("At bulunamadı.")
		}
		return domainhorse.Horse{}, apperr.Internal(fmt.Errorf("find horse by id: %w", pg.SanitizeErr(err)))
	}
	return h, nil
}

// FindByTJKNumber returns a horse by unique TJK number.
func (r *Repository) FindByTJKNumber(ctx context.Context, tjkNumber string) (domainhorse.Horse, error) {
	const q = `SELECT ` + horseColumns + ` FROM hrd_horses WHERE tjk_number = $1`
	h, err := scanHorse(r.db.QueryRow(ctx, q, tjkNumber))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainhorse.Horse{}, apperr.NotFound("At bulunamadı.")
		}
		return domainhorse.Horse{}, apperr.Internal(fmt.Errorf("find horse by tjk: %w", pg.SanitizeErr(err)))
	}
	return h, nil
}

// SearchByNormalizedPrefix searches horses by name_normalized prefix.
func (r *Repository) SearchByNormalizedPrefix(ctx context.Context, prefix string, limit int) ([]domainhorse.Horse, error) {
	const q = `
SELECT ` + horseColumns + `
FROM hrd_horses
WHERE name_normalized LIKE $1 || '%'
ORDER BY original_name ASC, tjk_number ASC, id ASC
LIMIT $2`
	rows, err := r.db.Query(ctx, q, prefix, limit)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("search horses: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()

	out := make([]domainhorse.Horse, 0, limit)
	for rows.Next() {
		h, err := scanHorse(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan horse: %w", pg.SanitizeErr(err)))
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate horses: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanHorse(row scannable) (domainhorse.Horse, error) {
	var h domainhorse.Horse
	var detail []byte
	err := row.Scan(
		&h.ID,
		&h.TJKNumber,
		&h.OriginalName,
		&h.NameNormalized,
		&h.BirthYear,
		&h.SireName,
		&h.DamName,
		&h.Breed,
		&h.Gender,
		&h.Coat,
		&detail,
		&h.LastSyncedAt,
		&h.LastSeenAt,
		&h.SourceUpdatedAt,
		&h.CreatedAt,
		&h.UpdatedAt,
	)
	if err != nil {
		return domainhorse.Horse{}, err
	}
	if len(detail) == 0 {
		h.Detail = json.RawMessage(`{}`)
	} else {
		h.Detail = json.RawMessage(detail)
	}
	return h, nil
}
