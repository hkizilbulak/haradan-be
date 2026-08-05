package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	appworker "github.com/hkizilbulak/haradan-be/internal/application/worker"
	"github.com/hkizilbulak/haradan-be/internal/config"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/email/resendemail"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/imageprocessor/tinifyprocessor"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/storage/s3storage"
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

	if cfg.StorageProvider != config.StorageProviderB2 {
		return fmt.Errorf("worker requires STORAGE_PROVIDER=b2")
	}
	if cfg.ImageProcessorProvider != config.ImageProcessorProviderTinify {
		return fmt.Errorf("worker requires IMAGE_PROCESSOR_PROVIDER=tinify")
	}

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
			TemplateID: cfg.ResendRegistrationVerificationTemplateID,
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
	scheduler, err := notificationWorker.NewExpiryScheduler(cfg.PackageExpirySchedulerInterval, log)
	if err != nil {
		return fmt.Errorf("notification expiry scheduler: %w", err)
	}
	supported := []domainmedia.JobType{
		domainmedia.JobValidateAndNormalize,
		domainmedia.JobGenerateVariant,
		domainmedia.JobNotificationFanoutAdvancedAdvert,
		domainmedia.JobNotificationFanoutUrgentAdvert,
		domainmedia.JobPackageExpiryReminderScan,
	}
	if emailJobsEnabled {
		supported = append(supported, domainmedia.JobEmailSendAdvertNotificationChunk, domainmedia.JobEmailSendPackageExpiryReminder)
	}

	runner, err := appworker.NewRunner(appworker.Config{
		WorkerID:              workerID,
		Concurrency:           cfg.WorkerConcurrency,
		PollInterval:          cfg.WorkerPollInterval,
		LeaseDuration:         cfg.WorkerLeaseDuration,
		JobTimeout:            cfg.WorkerJobTimeout,
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
	go scheduler.Run(runCtx)

	if err := runner.Run(runCtx); err != nil {
		return fmt.Errorf("runner: %w", err)
	}
	scheduler.Wait()
	log.Info("worker stopped", "workerId", workerID)
	return nil
}
