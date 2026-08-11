package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	appjobadmin "github.com/hkizilbulak/haradan-be/internal/application/jobadmin"
	"github.com/hkizilbulak/haradan-be/internal/application/jobscheduler"
	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	apptjk "github.com/hkizilbulak/haradan-be/internal/application/tjk"
	appworker "github.com/hkizilbulak/haradan-be/internal/application/worker"
	"github.com/hkizilbulak/haradan-be/internal/config"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/email/resendemail"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/imageprocessor/tinifyprocessor"
	pgjobdef "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/jobdef"
	pgtjk "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/tjk"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/storage/s3storage"
	tjkclient "github.com/hkizilbulak/haradan-be/internal/infrastructure/tjk"
	"github.com/hkizilbulak/haradan-be/internal/platform/database"
	applogger "github.com/hkizilbulak/haradan-be/internal/platform/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "worker exited with error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log := applogger.New(cfg.AppEnv)

	workerID := cfg.WorkerID
	if workerID == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "worker"
		}
		workerID = host + "-" + uuid.NewString()
	}
	log.Info("worker starting", "workerId", workerID, "env", cfg.AppEnv)
	mediaEnabled := cfg.StorageProvider == config.StorageProviderB2 &&
		cfg.ImageProcessorProvider == config.ImageProcessorProviderTinify
	log.Info("worker capabilities",
		"media", mediaEnabled,
		"email", cfg.EmailProvider == config.EmailProviderResend,
		"tjk", cfg.TJKEnabled,
	)

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

	var store appmedia.Storage
	var proc appmedia.ImageProcessor
	if mediaEnabled {
		store, err = s3storage.New(s3storage.Config{
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
		proc, err = tinifyprocessor.New(tinifyprocessor.Config{
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
	}

	mediaWorker, err := appmedia.NewPostgresWorker(db.Pool(), appmedia.WorkerConfig{
		Storage:   store,
		Processor: proc,
	})
	if err != nil {
		return fmt.Errorf("media worker: %w", err)
	}
	queue, err := appmedia.NewPostgresJobQueue(db.Pool())
	if err != nil {
		return fmt.Errorf("job queue: %w", err)
	}
	var notificationEmail appnotification.NotificationEmailSender
	emailJobsEnabled := false
	if cfg.EmailProvider == config.EmailProviderResend {
		sender, err := resendemail.New(resendemail.Config{
			APIKey: cfg.ResendAPIKey, BaseURL: cfg.ResendBaseURL, HTTPTimeout: cfg.EmailHTTPTimeout,
			FromEmail: cfg.FromEmail, FromName: cfg.FromName, FrontendURL: cfg.FrontendURL,
			WelcomeTemplateID:       cfg.ResendWelcomeTemplateID,
			ResetPasswordTemplateID: cfg.ResendResetPasswordTemplateID,
			TemplateID:              cfg.ResendRegistrationVerificationTemplateID,
		})
		if err != nil {
			return fmt.Errorf("notification email sender: %w", err)
		}
		notificationEmail = sender
		emailJobsEnabled = true
	}
	notificationWorker, err := appnotification.NewPostgresRuntimeWorker(db.Pool(), notificationEmail, cfg.FrontendURL, nil)
	if err != nil {
		return fmt.Errorf("notification runtime: %w", err)
	}

	jobRepo := pgjobdef.NewRepository(db.Pool())
	caps := appjobadmin.ProviderCapabilities{
		TJKEnabled:    cfg.TJKEnabled,
		B2Enabled:     cfg.StorageProvider == config.StorageProviderB2,
		TinifyEnabled: cfg.ImageProcessorProvider == config.ImageProcessorProviderTinify,
	}
	loc, err := time.LoadLocation(cfg.PackageExpiryTimezone)
	if err != nil {
		return fmt.Errorf("job scheduler timezone: %w", err)
	}
	defScheduler, err := jobscheduler.New(jobscheduler.Config{
		Definitions:     jobRepo,
		Enqueuer:        jobRepo,
		Capabilities:    caps,
		RefreshInterval: cfg.JobSchedulerRefreshInterval,
		Location:        loc,
		Logger:          log,
	})
	if err != nil {
		return fmt.Errorf("job definition scheduler: %w", err)
	}

	supported := supportedJobTypes(mediaEnabled, emailJobsEnabled)

	runner, err := appworker.NewRunner(appworker.Config{
		WorkerID:              workerID,
		Concurrency:           cfg.WorkerConcurrency,
		PollInterval:          cfg.WorkerPollInterval,
		LeaseDuration:         cfg.WorkerLeaseDuration,
		JobTimeout:            cfg.WorkerJobTimeout,
		MaxJobTimeout:         cfg.WorkerMaxJobTimeout,
		ShutdownTimeout:       cfg.WorkerShutdownTimeout,
		RetryBaseDelay:        cfg.WorkerRetryBaseDelay,
		RetryMaxDelay:         cfg.WorkerRetryMaxDelay,
		LeaseRecoveryInterval: cfg.WorkerLeaseRecoveryInterval,
		SupportedJobTypes:     supported,
		Queue:                 queue,
		Handler:               mediaWorker,
		NotificationHandler:   notificationWorker,
		Logger:                log,
		Backoff: appworker.Backoff{
			Base: cfg.WorkerRetryBaseDelay,
			Max:  cfg.WorkerRetryMaxDelay,
		},
	})
	if err != nil {
		return fmt.Errorf("runner: %w", err)
	}

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go defScheduler.Run(runCtx)
	if cfg.TJKEnabled {
		client, err := tjkclient.NewClient(tjkclient.Config{BaseURL: cfg.TJKBaseURL, HTTPTimeout: cfg.TJKHTTPTimeout, MaxBodyBytes: cfg.TJKMaxBodyBytes})
		if err != nil {
			return fmt.Errorf("TJK client: %w", err)
		}
		tjkWorker, err := apptjk.NewWorker(pgtjk.NewRepository(db.Pool()), tjkclient.WorkerAdapter{Client: client}, workerID)
		if err != nil {
			return fmt.Errorf("TJK worker: %w", err)
		}
		go runTJKWorker(runCtx, tjkWorker, cfg.WorkerLeaseDuration, cfg.WorkerPollInterval, cfg.TJKPageTimeout, log)
	}

	if err := runner.Run(runCtx); err != nil {
		return fmt.Errorf("runner: %w", err)
	}
	defScheduler.Wait()
	log.Info("worker stopped", "workerId", workerID)
	return nil
}

func supportedJobTypes(mediaEnabled, emailEnabled bool) []domainmedia.JobType {
	types := []domainmedia.JobType{
		domainmedia.JobNotificationFanoutPackageAdvert,
		domainmedia.JobNotificationFanoutAdvancedAdvert, // historical rows
		domainmedia.JobNotificationFanoutUrgentAdvert,
		domainmedia.JobPackageExpiryReminderScan,
	}
	if mediaEnabled {
		types = append(types,
			domainmedia.JobValidateAndNormalize,
			domainmedia.JobGenerateVariant,
			domainmedia.JobDeleteObjects,
			domainmedia.JobReconcile,
		)
	}
	if emailEnabled {
		types = append(types, domainmedia.JobEmailSendAdvertNotificationChunk, domainmedia.JobEmailSendPackageExpiryReminder)
	}
	return types
}

func runTJKWorker(ctx context.Context, worker *apptjk.Worker, lease, poll, jobTimeout time.Duration, log *slog.Logger) {
	for ctx.Err() == nil {
		jobCtx, cancel := context.WithTimeout(ctx, jobTimeout)
		claimed, err := worker.ProcessOnce(jobCtx, lease)
		cancel()
		if err != nil {
			log.Error("TJK job failed", "err", "dependency unavailable")
		}
		if claimed {
			continue
		}
		select {
		case <-ctx.Done():
		case <-time.After(poll):
		}
	}
}
