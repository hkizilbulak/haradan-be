package notification

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pgnotification "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/notification"
)

type pgRepo struct{ *pgnotification.Repository }

func (r pgRepo) WithTx(tx pgx.Tx) Repository {
	return pgRepo{r.Repository.WithTx(tx)}
}

// NewPostgresService constructs a notification Service backed by PostgreSQL.
func NewPostgresService(pool *pgxpool.Pool, users UserReader, clock Clock) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	return NewService(Config{
		Repo:  pgRepo{pgnotification.NewRepository(pool)},
		Users: users,
		Clock: clock,
	})
}

var _ Repository = pgRepo{}
