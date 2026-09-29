package communication_template

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	domain "github.com/hkizilbulak/haradan-be/internal/domain/communication_template"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository struct {
	db Querier
}

func NewRepository(db Querier) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetAll(ctx context.Context) ([]domain.Template, error) {
	const q = `
SELECT id, title, channel, content, subject, is_default, created_at, updated_at
FROM hrd_communication_templates
ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get all templates: %w", pg.SanitizeErr(err))
	}
	defer rows.Close()

	var out []domain.Template
	for rows.Next() {
		var t domain.Template
		if err := rows.Scan(&t.ID, &t.Title, &t.Channel, &t.Content, &t.Subject, &t.IsDefault, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan template: %w", pg.SanitizeErr(err))
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate templates: %w", pg.SanitizeErr(err))
	}
	return out, nil
}

func (r *Repository) Create(ctx context.Context, p domain.CreateParams) (*domain.Template, error) {
	const q = `
INSERT INTO hrd_communication_templates (title, channel, content, subject, is_default)
VALUES ($1, $2, $3, $4, false)
RETURNING id, title, channel, content, subject, is_default, created_at, updated_at`

	var t domain.Template
	err := r.db.QueryRow(ctx, q, p.Title, p.Channel, p.Content, p.Subject).Scan(
		&t.ID, &t.Title, &t.Channel, &t.Content, &t.Subject, &t.IsDefault, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert template: %w", pg.SanitizeErr(err))
	}
	return &t, nil
}

func (r *Repository) Update(ctx context.Context, id string, p domain.UpdateParams) (*domain.Template, error) {
	const q = `
UPDATE hrd_communication_templates
SET title = $2, channel = $3, content = $4, subject = $5, updated_at = NOW()
WHERE id = $1
RETURNING id, title, channel, content, subject, is_default, created_at, updated_at`

	var t domain.Template
	err := r.db.QueryRow(ctx, q, id, p.Title, p.Channel, p.Content, p.Subject).Scan(
		&t.ID, &t.Title, &t.Channel, &t.Content, &t.Subject, &t.IsDefault, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update template: %w", pg.SanitizeErr(err))
	}
	return &t, nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	const q = `
DELETE FROM hrd_communication_templates
WHERE id = $1`

	cmd, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete template: %w", pg.SanitizeErr(err))
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("template not found")
	}
	return nil
}
