package testutil

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenTestTx opens a pool from TEST_DATABASE_URL and begins a read-write transaction.
// The transaction is rolled back at cleanup. Skips when TEST_DATABASE_URL is unset.
// Never falls back to DATABASE_URL.
func OpenTestTx(t *testing.T) (context.Context, pgx.Tx, func()) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping repository integration test")
	}
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) != "" && dsn == strings.TrimSpace(os.Getenv("DATABASE_URL")) {
		t.Fatalf("TEST_DATABASE_URL must not equal DATABASE_URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		cancel()
		t.Fatalf("open test pool: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		pool.Close()
		cancel()
		t.Fatalf("begin test tx: %v", err)
	}

	cleanup := func() {
		_ = tx.Rollback(context.Background())
		pool.Close()
		cancel()
	}
	return ctx, tx, cleanup
}

// WithSavepoint runs fn inside a PostgreSQL SAVEPOINT and rolls it back afterward,
// so a statement-level error inside fn does not abort the outer test transaction.
func WithSavepoint(t *testing.T, ctx context.Context, tx pgx.Tx, fn func()) {
	t.Helper()
	sp := "sp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := tx.Exec(ctx, "SAVEPOINT "+sp); err != nil {
		t.Fatalf("savepoint begin: %v", err)
	}
	defer func() {
		if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp); err != nil {
			t.Fatalf("savepoint rollback: %v", err)
		}
		if _, err := tx.Exec(ctx, "RELEASE SAVEPOINT "+sp); err != nil {
			t.Fatalf("savepoint release: %v", err)
		}
	}()
	fn()
}
