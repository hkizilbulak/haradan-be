package auth

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pgauth "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/auth"
	pguser "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/user"
)

type pgUserRepo struct{ *pguser.Repository }

type pgSessionRepo struct{ *pgauth.Repository }

func (r pgSessionRepo) WithTx(tx pgx.Tx) SessionRepository {
	return pgSessionRepo{r.Repository.WithTx(tx)}
}

// NewPostgresService constructs a Service backed by PostgreSQL repositories.
func NewPostgresService(pool *pgxpool.Pool, cfg Config) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	cfg.Users = pgUserRepo{pguser.NewRepository(pool)}
	cfg.Sessions = pgSessionRepo{pgauth.NewRepository(pool)}
	cfg.UserTx = func(tx pgx.Tx) UserRepository {
		return pgUserRepo{pguser.NewRepository(tx)}
	}
	return NewService(cfg)
}

var (
	_ UserRepository    = pgUserRepo{}
	_ SessionRepository = pgSessionRepo{}
)
