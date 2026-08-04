package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	appcatalog "github.com/hkizilbulak/haradan-be/internal/application/catalog"
	appgeo "github.com/hkizilbulak/haradan-be/internal/application/geo"
	"github.com/hkizilbulak/haradan-be/internal/config"
	pgcatalog "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/catalog"
	pggeo "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/geo"
	"github.com/hkizilbulak/haradan-be/internal/platform/database"
	applogger "github.com/hkizilbulak/haradan-be/internal/platform/logger"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/password"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/token"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "api exited with error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := applogger.New(cfg.AppEnv)

	db, err := database.Open(context.Background(), database.Config{
		DatabaseURL:     cfg.DatabaseURL,
		MaxConns:        cfg.DBMaxConns,
		MinConns:        cfg.DBMinConns,
		MaxConnLifetime: cfg.DBMaxConnLifetime,
		MaxConnIdleTime: cfg.DBMaxConnIdleTime,
		HealthTimeout:   cfg.DBHealthTimeout,
	})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	hasher, err := password.NewHasher(password.Params{
		Time:    cfg.Argon2Time,
		Memory:  cfg.Argon2MemoryKiB,
		Threads: cfg.Argon2Threads,
		KeyLen:  cfg.Argon2KeyLen,
	})
	if err != nil {
		return fmt.Errorf("password hasher: %w", err)
	}
	tokenMgr, err := token.NewManager(token.Config{
		JWTSecret:          cfg.AuthJWTSecret,
		AccessTokenTTL:     cfg.AccessTokenTTL,
		RefreshAbsoluteTTL: cfg.RefreshAbsoluteTTL,
		RefreshIdleTTL:     cfg.RefreshIdleTTL,
	})
	if err != nil {
		return fmt.Errorf("token manager: %w", err)
	}
	authSvc, err := appauth.NewPostgresService(db.Pool(), appauth.Config{
		Hasher:            hasher,
		Tokens:            tokenMgr,
		EmailVerifyTTL:    cfg.EmailVerificationTTL,
		DummyPasswordHash: password.DummyHash(hasher),
	})
	if err != nil {
		return fmt.Errorf("auth service: %w", err)
	}

	geoRepo := pggeo.NewRepository(db.Pool())
	catalogRepo := pgcatalog.NewRepository(db.Pool())
	geoSvc := appgeo.NewService(geoRepo)
	catalogSvc := appcatalog.NewService(catalogRepo)

	srvHandler := handler.NewServer(log, db, geoSvc, catalogSvc, authSvc)
	engine := router.New(srvHandler, log, router.Options{AuthService: authSvc})

	httpServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      engine,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server starting", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info("shutdown signal received", "signal", sig.String())
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
	defer cancel()

	log.Info("http server shutting down")
	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	if err := <-errCh; err != nil {
		return fmt.Errorf("http server: %w", err)
	}

	log.Info("http server stopped")
	return nil
}
