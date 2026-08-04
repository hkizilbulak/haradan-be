package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/hkizilbulak/haradan-be/internal/config"
	applogger "github.com/hkizilbulak/haradan-be/internal/platform/logger"
	"github.com/hkizilbulak/haradan-be/internal/platform/migration"
	"github.com/hkizilbulak/haradan-be/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate exited with error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: migrate <up|status|version|down>")
	}
	command := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log := applogger.New(cfg.AppEnv)

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(context.Background(), cfg.DBHealthTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("ping database failed")
	}

	runner := &migration.Runner{
		DB:     db,
		FS:     migrations.FS,
		Logger: log,
	}

	ctx, cancelRun := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelRun()

	log.Info("migration starting", "command", command)
	if err := runner.Run(ctx, command); err != nil {
		return err
	}
	log.Info("migration finished", "command", command)
	return nil
}
