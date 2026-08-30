package comment

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domaincomment "github.com/hkizilbulak/haradan-be/internal/domain/comment"
	pgcomment "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/comment"
)

type pgRepo struct {
	*pgcomment.Repository
}

func (r pgRepo) FindAdvertStatus(ctx context.Context, advertID uuid.UUID) (AdvertStatusResult, error) {
	row, err := r.Repository.FindAdvertStatus(ctx, advertID)
	if err != nil {
		return AdvertStatusResult{}, err
	}
	return AdvertStatusResult{
		ID:        row.ID,
		Status:    row.Status,
		DeletedAt: row.DeletedAt,
	}, nil
}

func (r pgRepo) InsertComment(ctx context.Context, c domaincomment.Comment) error {
	return r.Repository.InsertComment(ctx, c)
}

func (r pgRepo) GetUserAuthorName(ctx context.Context, userID uuid.UUID) (string, error) {
	return r.Repository.GetUserAuthorName(ctx, userID)
}

func (r pgRepo) ListCommentsByAdvert(ctx context.Context, advertID uuid.UUID, limit, offset int) ([]CommentRow, int, error) {
	infraRows, total, err := r.Repository.ListCommentsByAdvert(ctx, advertID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	rows := make([]CommentRow, 0, len(infraRows))
	for _, ir := range infraRows {
		rows = append(rows, CommentRow{
			Comment:    ir.Comment,
			AuthorName: ir.AuthorName,
		})
	}
	return rows, total, nil
}

func (r pgRepo) AdminListComments(ctx context.Context, status *domaincomment.Status, limit, offset int) ([]CommentRow, int, error) {
	infraRows, total, err := r.Repository.AdminListComments(ctx, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	rows := make([]CommentRow, 0, len(infraRows))
	for _, ir := range infraRows {
		rows = append(rows, CommentRow{
			Comment:    ir.Comment,
			AuthorName: ir.AuthorName,
		})
	}
	return rows, total, nil
}

// NewPostgresService constructs a Service backed by PostgreSQL.
func NewPostgresService(pool *pgxpool.Pool, opts ...Option) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	repo := pgRepo{Repository: pgcomment.NewRepository(pool)}
	return NewService(repo, opts...), nil
}

var _ Repository = pgRepo{}
