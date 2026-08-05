package banner

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainbanner "github.com/hkizilbulak/haradan-be/internal/domain/banner"
	pgbanner "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/banner"
)

type pgRepo struct{ *pgbanner.Repository }

func (r pgRepo) WithTx(tx pgx.Tx) Repository { return pgRepo{r.Repository.WithTx(tx)} }
func (r pgRepo) Create(ctx context.Context, b domainbanner.Banner) error {
	return r.Repository.Create(ctx, b)
}
func (r pgRepo) GetByID(ctx context.Context, id uuid.UUID) (domainbanner.Banner, error) {
	return r.Repository.GetByID(ctx, id)
}
func (r pgRepo) LockByID(ctx context.Context, id uuid.UUID) (domainbanner.Banner, error) {
	return r.Repository.LockByID(ctx, id)
}
func (r pgRepo) List(ctx context.Context, f ListFilter) ([]domainbanner.Banner, error) {
	return r.Repository.List(ctx, pgbanner.ListFilter{Placement: f.Placement, Status: f.Status, Limit: f.Limit})
}
func (r pgRepo) ListActive(ctx context.Context, p domainbanner.Placement) ([]domainbanner.Banner, error) {
	return r.Repository.ListActive(ctx, p)
}
func (r pgRepo) UpdateOptimistic(ctx context.Context, b domainbanner.Banner, v int) (domainbanner.Banner, error) {
	return r.Repository.UpdateOptimistic(ctx, b, v)
}
func NewPostgresService(pool *pgxpool.Pool, media MediaReader, users UserReader, clock Clock) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	return NewService(Config{Repo: pgRepo{pgbanner.NewRepository(pool)}, Media: media, Users: users, Clock: clock})
}
