package migration

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"github.com/pressly/goose/v3"
)

// Runner executes Goose migrations against an opened *sql.DB.
type Runner struct {
	DB     *sql.DB
	FS     fs.FS
	Logger *slog.Logger
}

// Run validates embedded migrations then dispatches a Goose command.
func (r *Runner) Run(ctx context.Context, command string) error {
	if r == nil || r.DB == nil || r.FS == nil {
		return fmt.Errorf("migration runner is not configured")
	}
	logger := r.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if err := ValidateEmbeddedMigrations(r.FS); err != nil {
		return fmt.Errorf("migration safety validation failed: %w", err)
	}

	cmd := strings.ToLower(strings.TrimSpace(command))
	switch cmd {
	case "up", "status", "version":
		// allowed
	case "down":
		if strings.TrimSpace(os.Getenv("ALLOW_DESTRUCTIVE_MIGRATIONS")) != "true" {
			return fmt.Errorf("down requires ALLOW_DESTRUCTIVE_MIGRATIONS=true")
		}
	default:
		return fmt.Errorf("unknown migration command %q", command)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		r.DB,
		r.FS,
		goose.WithTableName(SchemaMigrationsTable),
	)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	switch cmd {
	case "up":
		_, err = provider.Up(ctx)
	case "down":
		_, err = provider.Down(ctx)
	case "status":
		_, err = provider.Status(ctx)
	case "version":
		_, err = provider.GetDBVersion(ctx)
	}
	if err != nil {
		return fmt.Errorf("migration %s failed: %w", cmd, err)
	}

	logger.Info("migration command completed", "command", cmd, "table", SchemaMigrationsTable)
	return nil
}
