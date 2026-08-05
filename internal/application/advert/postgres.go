package advert

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pgadvert "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/advert"
	pgcatalog "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/catalog"
	pggeo "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/geo"
	pghorse "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/horse"
	pguser "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/user"
)

type pgAdvertRepo struct{ *pgadvert.Repository }

func (r pgAdvertRepo) WithTx(tx pgx.Tx) Repository {
	return pgAdvertRepo{r.Repository.WithTx(tx)}
}

// NewPostgresService constructs a Service backed by PostgreSQL repositories.
func NewPostgresService(pool *pgxpool.Pool, cfg Config) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	repo := pgadvert.NewRepository(pool)
	cfg.Repo = pgAdvertRepo{repo}
	cfg.Public = repo
	cfg.Catalog = pgcatalog.NewRepository(pool)
	cfg.Geo = pggeo.NewRepository(pool)
	cfg.Horses = pghorse.NewRepository(pool)
	cfg.Users = pguser.NewRepository(pool)
	return NewService(cfg)
}

var _ Repository = pgAdvertRepo{}
