package studfarm

import (
	"github.com/google/uuid"
	"strings"
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainstudfarm "github.com/hkizilbulak/haradan-be/internal/domain/studfarm"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository implements stud farm persistence.
type Repository struct {
	db Querier
}

// NewRepository constructs a stud farm repository.
func NewRepository(db Querier) *Repository {
	return &Repository{db: db}
}

// List returns a paginated list of stud farms with their latest notes.
func (r *Repository) List(ctx context.Context, cursor *string, limit int) (domainstudfarm.ListResult, error) {
	args := []any{limit + 1}
	whereClause := ""
	
	if cursor != nil && *cursor != "" {
		cursorTime, err := time.Parse(time.RFC3339Nano, *cursor)
		if err != nil {
			return domainstudfarm.ListResult{}, apperr.Validation("invalid cursor format")
		}
		whereClause = "WHERE f.created_at < $2"
		args = append(args, cursorTime)
	}

	q := fmt.Sprintf(`
		SELECT 
			f.id, f.first_name, f.last_name, f.email, f.phone, f.location, f.created_at, f.updated_at,
			n.interview_date, n.interviewer_name, n.notes_url,
			COALESCE(c.cnt, 0) as interview_count
		FROM hrd_stud_farms f
		LEFT JOIN LATERAL (
			SELECT interview_date, interviewer_name, notes_url
			FROM hrd_stud_farm_notes
			WHERE stud_farm_id = f.id
			ORDER BY interview_date DESC
			LIMIT 1
		) n ON true
		LEFT JOIN LATERAL (
			SELECT COUNT(*) as cnt
			FROM hrd_stud_farm_notes
			WHERE stud_farm_id = f.id
		) c ON true
		%s
		ORDER BY f.created_at DESC
		LIMIT $1
	`, whereClause)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return domainstudfarm.ListResult{}, apperr.Internal(fmt.Errorf("query stud farms: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()

	var items []domainstudfarm.StudFarmListItem
	for rows.Next() {
		var item domainstudfarm.StudFarmListItem
		err := rows.Scan(
			&item.ID, &item.FirstName, &item.LastName, &item.Email, &item.Phone, &item.Location, &item.CreatedAt, &item.UpdatedAt,
			&item.LatestInterviewDate, &item.InterviewerName, &item.InterviewNotesURL, &item.InterviewCount,
		)
		if err != nil {
			return domainstudfarm.ListResult{}, apperr.Internal(fmt.Errorf("scan stud farm list item: %w", pg.SanitizeErr(err)))
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return domainstudfarm.ListResult{}, apperr.Internal(fmt.Errorf("rows err stud farms: %w", pg.SanitizeErr(err)))
	}

	hasMore := false
	var nextCursor *string
	
	if len(items) > limit {
		hasMore = true
		items = items[:limit]
		c := items[len(items)-1].CreatedAt.Format(time.RFC3339Nano)
		nextCursor = &c
	}

	return domainstudfarm.ListResult{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}


// Create inserts a new stud farm record.
func (r *Repository) Create(ctx context.Context, param domainstudfarm.CreateParam) (domainstudfarm.StudFarm, error) {
	id := uuid.New()
	now := time.Now().UTC()
	q := `
		INSERT INTO hrd_stud_farms (id, first_name, last_name, email, phone, location, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, first_name, last_name, email, phone, location, created_at, updated_at
	`
	var sf domainstudfarm.StudFarm
	err := r.db.QueryRow(ctx, q, id, param.FirstName, param.LastName, param.Email, param.Phone, param.Location, now, now).Scan(
		&sf.ID, &sf.FirstName, &sf.LastName, &sf.Email, &sf.Phone, &sf.Location, &sf.CreatedAt, &sf.UpdatedAt,
	)
	if err != nil {
		if err != nil && strings.Contains(err.Error(), "unique constraint") {
			return domainstudfarm.StudFarm{}, apperr.Conflict("email already exists")
		}
		return domainstudfarm.StudFarm{}, apperr.Internal(fmt.Errorf("insert stud farm: %w", pg.SanitizeErr(err)))
	}
	return sf, nil
}


func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, "DELETE FROM hrd_stud_farms WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}


func (r *Repository) AddNote(ctx context.Context, param domainstudfarm.NoteCreateParam) error {
	id := uuid.New()
	now := time.Now().UTC()
	q := `
		INSERT INTO hrd_stud_farm_notes (id, stud_farm_id, interviewer_name, interview_date, notes_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, q, id, param.StudFarmID, param.InterviewerName, param.InterviewDate, param.Notes, now)
	if err != nil {
		return err
	}
	return nil
}


func (r *Repository) ListNotes(ctx context.Context, studFarmId uuid.UUID) ([]domainstudfarm.Note, error) {
	q := `
		SELECT id, stud_farm_id, interviewer_name, interview_date, notes_url, created_at
		FROM hrd_stud_farm_notes
		WHERE stud_farm_id = $1
		ORDER BY interview_date DESC, created_at DESC
	`
	rows, err := r.db.Query(ctx, q, studFarmId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []domainstudfarm.Note
	for rows.Next() {
		var n domainstudfarm.Note
		if err := rows.Scan(&n.ID, &n.StudFarmID, &n.InterviewerName, &n.InterviewDate, &n.Notes, &n.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}
