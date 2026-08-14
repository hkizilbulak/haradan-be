package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	appadminuser "github.com/hkizilbulak/haradan-be/internal/application/adminuser"
	appadvert "github.com/hkizilbulak/haradan-be/internal/application/advert"
	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	appbanner "github.com/hkizilbulak/haradan-be/internal/application/banner"
	appcampaign "github.com/hkizilbulak/haradan-be/internal/application/campaign"
	appcatalog "github.com/hkizilbulak/haradan-be/internal/application/catalog"
	appcoupon "github.com/hkizilbulak/haradan-be/internal/application/coupon"
	appemail "github.com/hkizilbulak/haradan-be/internal/application/email"
	appfavorite "github.com/hkizilbulak/haradan-be/internal/application/favorite"
	appgeo "github.com/hkizilbulak/haradan-be/internal/application/geo"
	apphorse "github.com/hkizilbulak/haradan-be/internal/application/horse"
	appjobadmin "github.com/hkizilbulak/haradan-be/internal/application/jobadmin"
	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	apptjk "github.com/hkizilbulak/haradan-be/internal/application/tjk"
	"github.com/hkizilbulak/haradan-be/internal/config"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/email/resendemail"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/imageprocessor/tinifyprocessor"
	pgadminuser "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/adminuser"
	pgadvert "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/advert"
	pgcatalog "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/catalog"
	pgcoupon "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/coupon"
	pggeo "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/geo"
	pghorse "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/horse"
	pgjobdef "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/jobdef"
	pgmedia "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/media"
	pgtjk "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/tjk"
	pguser "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/user"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/storage/s3storage"
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
	var emailSender appauth.EmailSender = appauth.NoopEmailSender{}
	var emailDiscovery appemail.TemplateDiscovery
	switch cfg.EmailProvider {
	case config.EmailProviderUnconfigured:
		// keep NoopEmailSender
	case config.EmailProviderResend:
		sender, err := resendemail.New(resendemail.Config{
			APIKey:                  cfg.ResendAPIKey,
			BaseURL:                 cfg.ResendBaseURL,
			HTTPTimeout:             cfg.EmailHTTPTimeout,
			FromEmail:               cfg.FromEmail,
			FromName:                cfg.FromName,
			FrontendURL:             cfg.FrontendURL,
			WelcomeTemplateID:       cfg.ResendWelcomeTemplateID,
			ResetPasswordTemplateID: cfg.ResendResetPasswordTemplateID,
			TemplateID:              cfg.ResendRegistrationVerificationTemplateID,
		})
		if err != nil {
			return fmt.Errorf("email sender: %w", err)
		}
		emailSender = sender
		emailDiscovery = resendemail.Discovery{Sender: sender}
	default:
		return fmt.Errorf("email provider is not supported")
	}

	authSvc, err := appauth.NewPostgresService(db.Pool(), appauth.Config{
		Hasher:            hasher,
		Tokens:            tokenMgr,
		EmailSender:       emailSender,
		EmailVerifyTTL:    cfg.EmailVerificationTTL,
		DummyPasswordHash: password.DummyHash(hasher),
	})
	if err != nil {
		return fmt.Errorf("auth service: %w", err)
	}
	adminUserSvc, err := appadminuser.NewService(appadminuser.Config{
		Repository:      pgadminuser.NewRepository(db.Pool()),
		Hasher:          hasher,
		EmailSender:     emailSender,
		EmailConfigured: cfg.EmailProvider == config.EmailProviderResend,
		InvitationTTL:   cfg.EmailVerificationTTL,
	})
	if err != nil {
		return fmt.Errorf("admin user service: %w", err)
	}

	geoRepo := pggeo.NewRepository(db.Pool())
	catalogRepo := pgcatalog.NewRepository(db.Pool())
	horseRepo := pghorse.NewRepository(db.Pool())
	geoSvc := appgeo.NewService(geoRepo)
	catalogSvc := appcatalog.NewService(catalogRepo)
	horseSvc := apphorse.NewService(horseRepo)
	// Storage uses B2 when STORAGE_PROVIDER=b2; otherwise UnconfiguredStorage.
	// ImageProcessor uses Tinify when IMAGE_PROCESSOR_PROVIDER=tinify; otherwise Unconfigured.
	var mediaStorage appmedia.Storage = appmedia.UnconfiguredStorage{}
	switch cfg.StorageProvider {
	case config.StorageProviderUnconfigured:
		// keep UnconfiguredStorage
	case config.StorageProviderB2:
		store, err := s3storage.New(s3storage.Config{
			Endpoint:  cfg.S3Endpoint,
			Region:    cfg.S3Region,
			Bucket:    cfg.S3Bucket,
			AccessKey: cfg.S3AccessKey,
			SecretKey: cfg.S3SecretKey,
			BasePath:  cfg.S3BasePath,
		})
		if err != nil {
			return fmt.Errorf("storage: %w", err)
		}
		mediaStorage = store
	default:
		return fmt.Errorf("storage provider is not supported")
	}

	var mediaProcessor appmedia.ImageProcessor = appmedia.UnconfiguredImageProcessor{}
	switch cfg.ImageProcessorProvider {
	case config.ImageProcessorProviderUnconfigured:
		// keep UnconfiguredImageProcessor
	case config.ImageProcessorProviderTinify:
		proc, err := tinifyprocessor.New(tinifyprocessor.Config{
			APIKey:      cfg.TinifyAPIKey,
			BaseURL:     cfg.TinifyBaseURL,
			HTTPTimeout: cfg.TinifyHTTPTimeout,
			Profiles: map[string]tinifyprocessor.ProfileConfig{
				domainmedia.ProfileDetail:   {Width: cfg.MediaProfileDetailW, Height: cfg.MediaProfileDetailH},
				domainmedia.ProfileHomepage: {Width: cfg.MediaProfileHomepageW, Height: cfg.MediaProfileHomepageH},
				domainmedia.ProfileSearch:   {Width: cfg.MediaProfileSearchW, Height: cfg.MediaProfileSearchH},
			},
		})
		if err != nil {
			return fmt.Errorf("image processor: %w", err)
		}
		mediaProcessor = proc
	default:
		return fmt.Errorf("image processor provider is not supported")
	}

	mediaSvc, err := appmedia.NewPostgresService(db.Pool(), appmedia.Config{
		Storage:             mediaStorage,
		Processor:           mediaProcessor,
		AllowedContentTypes: cfg.MediaAllowedContentTypes,
		MaxByteSize:         cfg.MediaMaxByteSize,
		UploadURLTTL:        cfg.MediaUploadURLTTL,
	})
	if err != nil {
		return fmt.Errorf("media service: %w", err)
	}
	favoriteSvc, err := appfavorite.NewPostgresService(db.Pool(), appfavorite.Config{})
	if err != nil {
		return fmt.Errorf("favorite service: %w", err)
	}

	advertRepo := pgadvert.NewRepository(db.Pool())
	userRepo := pguser.NewRepository(db.Pool())
	mediaRepo := pgmedia.NewRepository(db.Pool())
	campaignPackages := appcampaign.NewPostgresPackageLookup(db.Pool())

	notificationEmitter, err := appnotification.NewPostgresEmitter(db.Pool(), cfg.FrontendURL, nil)
	if err != nil {
		return fmt.Errorf("notification emitter: %w", err)
	}
	advertSvc, err := appadvert.NewPostgresService(db.Pool(), appadvert.Config{
		Notifications: notificationEmitter,
	})
	if err != nil {
		return fmt.Errorf("advert service: %w", err)
	}
	packagingSvc, err := apppackaging.NewPostgresService(db.Pool(), advertRepo, userRepo, nil, notificationEmitter)
	if err != nil {
		return fmt.Errorf("packaging service: %w", err)
	}
	campaignSvc, err := appcampaign.NewPostgresService(db.Pool(), campaignPackages, mediaRepo, userRepo, nil)
	if err != nil {
		return fmt.Errorf("campaign service: %w", err)
	}
	bannerSvc, err := appbanner.NewPostgresService(db.Pool(), mediaRepo, userRepo, nil)
	if err != nil {
		return fmt.Errorf("banner service: %w", err)
	}
	notificationSvc, err := appnotification.NewPostgresService(db.Pool(), userRepo, nil)
	if err != nil {
		return fmt.Errorf("notification service: %w", err)
	}
	notificationInboxSvc, err := appnotification.NewPostgresUserNotificationService(db.Pool(), nil)
	if err != nil {
		return fmt.Errorf("notification inbox service: %w", err)
	}
	tjkSvc, err := apptjk.NewService(apptjk.Config{
		Repo:    pgtjk.NewRepository(db.Pool()),
		Enabled: cfg.TJKEnabled,
	})
	if err != nil {
		return fmt.Errorf("TJK service: %w", err)
	}
	jobAdminSvc, err := appjobadmin.NewService(appjobadmin.Config{
		Repo:  pgjobdef.NewRepository(db.Pool()),
		Users: userRepo,
		Caps: appjobadmin.ProviderCapabilities{
			TJKEnabled:    cfg.TJKEnabled,
			B2Enabled:     cfg.StorageProvider == config.StorageProviderB2,
			TinifyEnabled: cfg.ImageProcessorProvider == config.ImageProcessorProviderTinify,
		},
	})
	if err != nil {
		return fmt.Errorf("job admin service: %w", err)
	}

	couponSvc, err := appcoupon.NewService(appcoupon.Config{
		Repo: pgcoupon.NewRepository(db.Pool()),
	})
	if err != nil {
		return fmt.Errorf("coupon service: %w", err)
	}

	srvHandler := handler.NewServer(
		log, db, geoSvc, catalogSvc, horseSvc, advertSvc, mediaSvc, favoriteSvc,
		packagingSvc, campaignSvc, campaignPackages, notificationSvc, authSvc, notificationInboxSvc,
	).WithPublicMediaDelivery(mediaSvc).
		WithBannerService(bannerSvc, mediaRepo).
		WithAdminUserService(adminUserSvc).
		WithTJKService(tjkSvc).
		WithEmailTemplateDiscovery(emailDiscovery).
		WithJobAdminService(jobAdminSvc)
	engine := router.New(srvHandler, log, router.Options{
		AuthService:        authSvc,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		CORSAllowLoopback:  cfg.CORSAllowLoopback,
	})

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
