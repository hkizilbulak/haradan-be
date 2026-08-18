// Package comment implements PostgreSQL persistence for hrd_advert_comments.
package comment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincomment "github.com/hkizilbulak/haradan-be/internal/domain/comment"
)

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository persists comments with pgx.
type Repository struct {
	db Querier
}

// NewRepository constructs a PostgreSQL comment repository.
func NewRepository(db Querier) *Repository {
	return &Repository{db: db}
}

// AdvertStatusRow is the advert projection needed for comment status check.
type AdvertStatusRow struct {
	ID        uuid.UUID
	Status    string
	DeletedAt *time.Time
}

// CommentRow is the infra comment row with author display name.
type CommentRow struct {
	Comment    domaincomment.Comment
	AuthorName string
}

// FindAdvertStatus checks whether the advert exists and returns its status.
func (r *Repository) FindAdvertStatus(ctx context.Context, advertID uuid.UUID) (AdvertStatusRow, error) {
	const query = `
		SELECT id, status, deleted_at
		FROM hrd_adverts
		WHERE id = $1
	`
	var row AdvertStatusRow
	err := r.db.QueryRow(ctx, query, advertID).Scan(&row.ID, &row.Status, &row.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdvertStatusRow{}, apperr.NotFound("advert not found")
	}
	if err != nil {
		return AdvertStatusRow{}, fmt.Errorf("query advert status: %w", err)
	}
	return row, nil
}

// InsertComment persists a comment row.
func (r *Repository) InsertComment(ctx context.Context, c domaincomment.Comment) error {
	const query = `
		INSERT INTO hrd_advert_comments (
			id, advert_id, user_id, content, status, created_at, updated_at, deleted_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
	`
	_, err := r.db.Exec(
		ctx, query,
		c.ID, c.AdvertID, c.UserID, c.Content, string(c.Status),
		c.CreatedAt, c.UpdatedAt, c.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("insert advert comment: %w", err)
	}
	return nil
}

// GetUserAuthorName fetches the user's first and last name or email prefix as display name.
func (r *Repository) GetUserAuthorName(ctx context.Context, userID uuid.UUID) (string, error) {
	const query = `
		SELECT first_name, last_name, email
		FROM hrd_users
		WHERE id = $1
	`
	var firstName, lastName, email string
	err := r.db.QueryRow(ctx, query, userID).Scan(&firstName, &lastName, &email)
	if err != nil {
		return "Kullanıcı", nil
	}

	if firstName != "" || lastName != "" {
		if lastName != "" {
			// Mask last name for privacy: "Ahmet Y."
			runes := []rune(lastName)
			return fmt.Sprintf("%s %s.", firstName, string(runes[0])), nil
		}
		return firstName, nil
	}
	return "Kullanıcı", nil
}

// ListCommentsByAdvert returns published comments with author names for an advert.
func (r *Repository) ListCommentsByAdvert(ctx context.Context, advertID uuid.UUID, limit, offset int) ([]CommentRow, int, error) {
	const countQuery = `
		SELECT COUNT(*)
		FROM hrd_advert_comments
		WHERE advert_id = $1 AND deleted_at IS NULL AND status = 'PUBLISHED'
	`
	var total int
	err := r.db.QueryRow(ctx, countQuery, advertID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count advert comments: %w", err)
	}

	if total == 0 {
		return []CommentRow{}, 0, nil
	}

	const selectQuery = `
		SELECT c.id, c.advert_id, c.user_id, c.content, c.status, c.created_at, c.updated_at, c.deleted_at,
		       COALESCE(u.first_name, ''), COALESCE(u.last_name, ''), COALESCE(u.email, '')
		FROM hrd_advert_comments c
		LEFT JOIN hrd_users u ON u.id = c.user_id
		WHERE c.advert_id = $1 AND c.deleted_at IS NULL AND c.status = 'PUBLISHED'
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, selectQuery, advertID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query advert comments: %w", err)
	}
	defer rows.Close()

	var result []CommentRow
	for rows.Next() {
		var (
			c domaincomment.Comment
			st string
			fn, ln, em string
		)
		err := rows.Scan(
			&c.ID, &c.AdvertID, &c.UserID, &c.Content, &st, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
			&fn, &ln, &em,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan advert comment: %w", err)
		}
		c.Status = domaincomment.Status(st)

		authorName := "Kullanıcı"
		if fn != "" || ln != "" {
			if ln != "" {
				runes := []rune(ln)
				authorName = fmt.Sprintf("%s %s.", fn, string(runes[0]))
			} else {
				authorName = fn
			}
		}

		result = append(result, CommentRow{
			Comment:    c,
			AuthorName: authorName,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows advert comments: %w", err)
	}

	return result, total, nil
}
