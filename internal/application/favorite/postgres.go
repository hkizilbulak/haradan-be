package favorite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domainfavorite "github.com/hkizilbulak/haradan-be/internal/domain/favorite"
	pgfavorite "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/favorite"
)

type pgRepo struct{ *pgfavorite.Repository }

func mapAdvert(row pgfavorite.AdvertRow) AdvertSnapshot {
	return AdvertSnapshot{
		ID:               row.ID,
		Status:           row.Status,
		DeletedAt:        row.DeletedAt,
		Title:            row.Title,
		PublishedAt:      row.PublishedAt,
		CategoryID:       row.CategoryID,
		DistrictID:       row.DistrictID,
		ProvinceID:       row.ProvinceID,
		HorseID:          row.HorseID,
		PriceAmountMinor: row.PriceAmountMinor,
		PriceCurrency:    row.PriceCurrency,
	}
}

func (r pgRepo) FindAdvertForFavoriteLookup(ctx context.Context, advertID uuid.UUID) (AdvertSnapshot, error) {
	row, err := r.Repository.FindAdvertForFavoriteLookup(ctx, advertID)
	if err != nil {
		return AdvertSnapshot{}, err
	}
	return mapAdvert(row), nil
}

func (r pgRepo) InsertFavorite(ctx context.Context, fav domainfavorite.Favorite) error {
	err := r.Repository.InsertFavorite(ctx, fav)
	if errors.Is(err, domainfavorite.ErrDuplicate) {
		return ErrDuplicateFavorite
	}
	return err
}

func (r pgRepo) DeleteFavorite(ctx context.Context, userID, advertID uuid.UUID) error {
	return r.Repository.DeleteFavorite(ctx, userID, advertID)
}

func (r pgRepo) ListFavoritesByUser(
	ctx context.Context,
	userID uuid.UUID,
	afterCreatedAt *time.Time,
	afterID *uuid.UUID,
	limit int,
) ([]ListRow, error) {
	rows, err := r.Repository.ListFavoritesByUser(ctx, userID, afterCreatedAt, afterID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ListRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ListRow{Favorite: row.Favorite, Advert: mapAdvert(row.Advert)})
	}
	return out, nil
}

// NewPostgresService constructs a Service backed by PostgreSQL.
func NewPostgresService(pool *pgxpool.Pool, cfg Config) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	cfg.Repo = pgRepo{pgfavorite.NewRepository(pool)}
	return NewService(cfg)
}

var _ Repository = pgRepo{}
