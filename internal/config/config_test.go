package config_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hkizilbulak/haradan-be/internal/config"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

func TestLoadDefaults(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.AppEnv != "development" || cfg.HTTPAddr != ":8080" {
		t.Fatalf("unexpected base cfg: %+v", cfg)
	}
	if cfg.DBMaxConns != 10 || cfg.DBMinConns != 2 {
		t.Fatalf("unexpected pool defaults: %+v", cfg)
	}
	if cfg.DBMaxConnLifetime != 30*time.Minute || cfg.DBMaxConnIdleTime != 5*time.Minute {
		t.Fatalf("unexpected pool durations: %+v", cfg)
	}
	if cfg.DBHealthTimeout != 2*time.Second {
		t.Fatalf("unexpected health timeout: %v", cfg.DBHealthTimeout)
	}
	if cfg.TJKEnabled || cfg.TJKBaseURL != "" {
		t.Fatalf("TJK should be disabled by default: enabled=%v url=%q", cfg.TJKEnabled, cfg.TJKBaseURL)
	}
	if cfg.TJKHTTPTimeout != 60*time.Second {
		t.Fatalf("TJK timeout default = %v", cfg.TJKHTTPTimeout)
	}
}

func TestLoadTJKEnabledDefaultsBaseURL(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("TJK_ENABLED", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.TJKEnabled {
		t.Fatal("expected TJK enabled")
	}
	if cfg.TJKBaseURL != "https://www.tjk.org" {
		t.Fatalf("base URL = %q", cfg.TJKBaseURL)
	}
	if cfg.TJKHTTPTimeout != 60*time.Second {
		t.Fatalf("timeout = %v", cfg.TJKHTTPTimeout)
	}
}

func TestLoadTJKExplicitBaseURL(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("TJK_BASE_URL", "https://tjk.example.test")
	t.Setenv("TJK_HTTP_TIMEOUT", "45s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.TJKEnabled {
		t.Fatal("expected TJK enabled when base URL set")
	}
	if cfg.TJKBaseURL != "https://tjk.example.test" {
		t.Fatalf("base URL = %q", cfg.TJKBaseURL)
	}
	if cfg.TJKHTTPTimeout != 45*time.Second {
		t.Fatalf("timeout = %v", cfg.TJKHTTPTimeout)
	}
}

func TestLoadValidTimeoutsAndDB(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("HTTP_READ_TIMEOUT", "3s")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("DB_MAX_CONNS", "20")
	t.Setenv("DB_MIN_CONNS", "5")
	t.Setenv("DB_MAX_CONN_LIFETIME", "10m")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "1m")
	t.Setenv("DB_HEALTH_TIMEOUT", "1500ms")
	t.Setenv("AUTH_JWT_SECRET", "production-test-secret-value")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DBMaxConns != 20 || cfg.DBMinConns != 5 {
		t.Fatalf("pool parse failed: %+v", cfg)
	}
	if cfg.DBHealthTimeout != 1500*time.Millisecond {
		t.Fatalf("health timeout=%v", cfg.DBHealthTimeout)
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("HTTP_READ_TIMEOUT", "not-a-duration")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected invalid duration error")
	}
}

func TestLoadEmptyHTTPAddr(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("HTTP_ADDR", "   ")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected empty HTTP_ADDR error")
	}
}

func TestLoadEmptyDatabaseURL(t *testing.T) {
	clearConfigEnv(t)
	if _, err := config.Load(); err == nil {
		t.Fatal("expected empty DATABASE_URL error")
	}
}

func TestLoadInvalidNumericAndMinMax(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("DB_MAX_CONNS", "abc")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected invalid numeric error")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("DB_MAX_CONNS", "2")
	t.Setenv("DB_MIN_CONNS", "5")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected min>max error")
	}
}

func TestLoadErrorDoesNotLeakSecret(t *testing.T) {
	clearConfigEnv(t)
	secret := "postgres://supersecret-user:supersecret-pass@db.example:5432/haradan"
	t.Setenv("DATABASE_URL", secret)
	t.Setenv("DB_MAX_CONN_LIFETIME", "bad")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "supersecret") || strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked secret: %v", err)
	}
}

func TestLoadAuthDefaultsAndProductionSecret(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AccessTokenTTL != 15*time.Minute || cfg.AuthJWTSecret != "" {
		t.Fatalf("unexpected auth defaults: %+v", cfg)
	}

	clearConfigEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected missing AUTH_JWT_SECRET error")
	}

	clearConfigEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("AUTH_JWT_SECRET", "production-test-secret-value")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthJWTSecret != "production-test-secret-value" {
		t.Fatal("secret not loaded")
	}
}

func TestLoadAuthIdleExceedsAbsolute(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("AUTH_REFRESH_ABSOLUTE_TTL", "1h")
	t.Setenv("AUTH_REFRESH_IDLE_TTL", "2h")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected idle>absolute error")
	}
}

func TestLoadZeroOrNegativeAuthTTL(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("AUTH_ACCESS_TOKEN_TTL", "0s")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected zero access TTL error")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("AUTH_REFRESH_ABSOLUTE_TTL", "-1h")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected negative refresh TTL error")
	}
}

// TestLoadMediaSettings covers the media upload knobs: unset means "not
// configured", never an invented allowlist or byte ceiling.
func TestLoadMediaSettings(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.MediaAllowedContentTypes) != 0 || cfg.MediaMaxByteSize != domainmedia.MaxUploadBytes {
		t.Fatalf("media limits: types=%v max=%d", cfg.MediaAllowedContentTypes, cfg.MediaMaxByteSize)
	}
	if cfg.MediaUploadURLTTL != 15*time.Minute {
		t.Fatalf("MediaUploadURLTTL=%v", cfg.MediaUploadURLTTL)
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("MEDIA_ALLOWED_CONTENT_TYPES", " image/jpeg , image/png ,, ")
	t.Setenv("MEDIA_MAX_BYTE_SIZE", "5242880")
	t.Setenv("MEDIA_UPLOAD_URL_TTL", "5m")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.MediaAllowedContentTypes) != 2 ||
		cfg.MediaAllowedContentTypes[0] != "image/jpeg" ||
		cfg.MediaAllowedContentTypes[1] != "image/png" {
		t.Fatalf("MediaAllowedContentTypes=%v", cfg.MediaAllowedContentTypes)
	}
	if cfg.MediaMaxByteSize != 5242880 || cfg.MediaUploadURLTTL != 5*time.Minute {
		t.Fatalf("media overrides not applied: %+v", cfg)
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("MEDIA_MAX_BYTE_SIZE", "-1")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected negative MEDIA_MAX_BYTE_SIZE error")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("MEDIA_MAX_BYTE_SIZE", "abc")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected invalid MEDIA_MAX_BYTE_SIZE error")
	}
}

func TestLoadStorageProviderDefaultsAndB2(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorageProvider != config.StorageProviderUnconfigured {
		t.Fatalf("StorageProvider=%q", cfg.StorageProvider)
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("STORAGE_PROVIDER", "unconfigured")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorageProvider != config.StorageProviderUnconfigured {
		t.Fatalf("StorageProvider=%q", cfg.StorageProvider)
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("STORAGE_PROVIDER", "backblaze")
	t.Setenv("S3_ENDPOINT", "https://example.invalid")
	t.Setenv("S3_REGION", "eu-central-003")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("S3_ACCESS_KEY", "access-key-value")
	t.Setenv("S3_SECRET_KEY", "secret-key-value-do-not-leak")
	t.Setenv("S3_BASE_PATH", "/media/prod/")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorageProvider != config.StorageProviderB2 {
		t.Fatalf("StorageProvider=%q", cfg.StorageProvider)
	}
	if cfg.S3BasePath != "media/prod" {
		t.Fatalf("S3BasePath=%q", cfg.S3BasePath)
	}
	if cfg.MediaProfileDetailW != domainmedia.ProfileDetailWidth {
		t.Fatalf("default DETAIL width=%d", cfg.MediaProfileDetailW)
	}
	if cfg.MediaPublicBaseURL != "" {
		t.Fatalf("MEDIA_PUBLIC_BASE_URL must be optional for b2, got %q", cfg.MediaPublicBaseURL)
	}
}

func TestLoadStorageProviderB2RequiresFields(t *testing.T) {
	required := []string{"S3_ENDPOINT", "S3_REGION", "S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY"}
	for _, missing := range required {
		clearConfigEnv(t)
		t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
		t.Setenv("STORAGE_PROVIDER", "b2")
		t.Setenv("S3_ENDPOINT", "https://example.invalid")
		t.Setenv("S3_REGION", "eu-central-003")
		t.Setenv("S3_BUCKET", "bucket")
		t.Setenv("S3_ACCESS_KEY", "access-key-value")
		t.Setenv("S3_SECRET_KEY", "secret-key-value-do-not-leak")
		_ = os.Unsetenv(missing)
		_, err := config.Load()
		if err == nil {
			t.Fatalf("expected error when %s missing", missing)
		}
		if strings.Contains(err.Error(), "secret-key-value") || strings.Contains(err.Error(), "access-key-value") {
			t.Fatalf("error leaked credential for missing %s: %v", missing, err)
		}
	}
}

func TestLoadStorageUnknownProviderAndBasePath(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("STORAGE_PROVIDER", "minio")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected unknown provider error")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("S3_BASE_PATH", "media/../secret")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected base path traversal error")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("S3_BASE_PATH", `media\bad`)
	if _, err := config.Load(); err == nil {
		t.Fatal("expected base path backslash error")
	}
}

func TestLoadImageProcessorProviderDefaultsAndTinify(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ImageProcessorProvider != config.ImageProcessorProviderUnconfigured {
		t.Fatalf("ImageProcessorProvider=%q", cfg.ImageProcessorProvider)
	}
	if cfg.TinifyBaseURL != "https://api.tinify.com" {
		t.Fatalf("TinifyBaseURL=%q", cfg.TinifyBaseURL)
	}
	if cfg.TinifyHTTPTimeout != 30*time.Second {
		t.Fatalf("TinifyHTTPTimeout=%v", cfg.TinifyHTTPTimeout)
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("IMAGE_PROCESSOR_PROVIDER", "unconfigured")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ImageProcessorProvider != config.ImageProcessorProviderUnconfigured {
		t.Fatalf("ImageProcessorProvider=%q", cfg.ImageProcessorProvider)
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("IMAGE_PROCESSOR_PROVIDER", "TINIFY")
	t.Setenv("TINIFY_API_KEY", "test-api-key-do-not-leak")
	t.Setenv("MEDIA_PROFILE_DETAIL_WIDTH", "10")
	t.Setenv("MEDIA_PROFILE_DETAIL_HEIGHT", "20")
	t.Setenv("MEDIA_PROFILE_HOMEPAGE_WIDTH", "30")
	t.Setenv("MEDIA_PROFILE_HOMEPAGE_HEIGHT", "40")
	t.Setenv("MEDIA_PROFILE_SEARCH_WIDTH", "50")
	t.Setenv("MEDIA_PROFILE_SEARCH_HEIGHT", "60")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ImageProcessorProvider != config.ImageProcessorProviderTinify {
		t.Fatalf("ImageProcessorProvider=%q", cfg.ImageProcessorProvider)
	}
	if cfg.MediaProfileDetailW != 10 || cfg.MediaProfileSearchH != 60 {
		t.Fatalf("unexpected profile dims detail=%dx%d searchH=%d", cfg.MediaProfileDetailW, cfg.MediaProfileDetailH, cfg.MediaProfileSearchH)
	}
}

func TestLoadImageProcessorTinifyValidation(t *testing.T) {
	setTinifyBase := func(t *testing.T) {
		t.Helper()
		t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
		t.Setenv("IMAGE_PROCESSOR_PROVIDER", "tinify")
		t.Setenv("TINIFY_API_KEY", "test-api-key-do-not-leak")
		t.Setenv("MEDIA_PROFILE_DETAIL_WIDTH", "10")
		t.Setenv("MEDIA_PROFILE_DETAIL_HEIGHT", "20")
		t.Setenv("MEDIA_PROFILE_HOMEPAGE_WIDTH", "30")
		t.Setenv("MEDIA_PROFILE_HOMEPAGE_HEIGHT", "40")
		t.Setenv("MEDIA_PROFILE_SEARCH_WIDTH", "50")
		t.Setenv("MEDIA_PROFILE_SEARCH_HEIGHT", "60")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("IMAGE_PROCESSOR_PROVIDER", "imaginary")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected unknown provider error")
	}

	clearConfigEnv(t)
	setTinifyBase(t)
	_ = os.Unsetenv("TINIFY_API_KEY")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected missing API key error")
	} else if strings.Contains(err.Error(), "test-api-key") {
		t.Fatalf("error leaked api key: %v", err)
	}

	dims := []string{
		"MEDIA_PROFILE_DETAIL_WIDTH",
		"MEDIA_PROFILE_DETAIL_HEIGHT",
		"MEDIA_PROFILE_HOMEPAGE_WIDTH",
		"MEDIA_PROFILE_HOMEPAGE_HEIGHT",
		"MEDIA_PROFILE_SEARCH_WIDTH",
		"MEDIA_PROFILE_SEARCH_HEIGHT",
	}
	for _, missing := range dims {
		clearConfigEnv(t)
		setTinifyBase(t)
		_ = os.Unsetenv(missing)
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("missing %s should fall back to defaults: %v", missing, err)
		}
		if cfg.MediaProfileDetailW <= 0 || cfg.MediaProfileSearchH <= 0 {
			t.Fatalf("defaults not applied after unsetting %s: %+v", missing, cfg)
		}
	}

	clearConfigEnv(t)
	setTinifyBase(t)
	t.Setenv("MEDIA_PROFILE_DETAIL_WIDTH", "0")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected zero width error")
	}

	clearConfigEnv(t)
	setTinifyBase(t)
	t.Setenv("TINIFY_HTTP_TIMEOUT", "not-a-duration")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected invalid timeout error")
	}

	clearConfigEnv(t)
	setTinifyBase(t)
	t.Setenv("TINIFY_BASE_URL", "http://api.tinify.com")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected http base URL rejection")
	}

	clearConfigEnv(t)
	setTinifyBase(t)
	t.Setenv("TINIFY_BASE_URL", "https://user:pass@api.tinify.com")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected userinfo rejection")
	}

	clearConfigEnv(t)
	setTinifyBase(t)
	t.Setenv("TINIFY_BASE_URL", "https://api.tinify.com/v1")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected path rejection")
	}

	clearConfigEnv(t)
	setTinifyBase(t)
	t.Setenv("MEDIA_ALLOWED_CONTENT_TYPES", "image/jpeg,image/gif")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected unsupported content type error")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("IMAGE_PROCESSOR_PROVIDER", "unconfigured")
	t.Setenv("MEDIA_ALLOWED_CONTENT_TYPES", "image/jpeg,image/gif")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.MediaAllowedContentTypes) != 2 {
		t.Fatalf("unconfigured provider should keep media allowlist intact: %v", cfg.MediaAllowedContentTypes)
	}
}

func TestLoadWorkerDefaultsAndValidation(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkerConcurrency != 2 || cfg.WorkerPollInterval != time.Second {
		t.Fatalf("defaults concurrency=%d poll=%v", cfg.WorkerConcurrency, cfg.WorkerPollInterval)
	}
	if cfg.WorkerLeaseDuration != 2*time.Minute || cfg.WorkerJobTimeout != 60*time.Second {
		t.Fatalf("lease=%v timeout=%v", cfg.WorkerLeaseDuration, cfg.WorkerJobTimeout)
	}
	if cfg.WorkerID != "" {
		t.Fatalf("WorkerID should default empty, got %q", cfg.WorkerID)
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("WORKER_CONCURRENCY", "0")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected concurrency validation error")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("WORKER_JOB_TIMEOUT", "3m")
	t.Setenv("WORKER_LEASE_DURATION", "2m")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected job timeout < lease validation")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("WORKER_RETRY_BASE_DELAY", "10s")
	t.Setenv("WORKER_RETRY_MAX_DELAY", "5s")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected retry base <= max validation")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("WORKER_ID", "   ")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected empty WORKER_ID rejection")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("WORKER_ID", "worker-a")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkerID != "worker-a" {
		t.Fatalf("WorkerID=%q", cfg.WorkerID)
	}
}

func TestLoadEmailProviderDefaultsAndResend(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EmailProvider != config.EmailProviderUnconfigured {
		t.Fatalf("EmailProvider=%q", cfg.EmailProvider)
	}
	if cfg.ResendBaseURL != "https://api.resend.com" {
		t.Fatalf("ResendBaseURL=%q", cfg.ResendBaseURL)
	}
	if cfg.EmailHTTPTimeout != 30*time.Second {
		t.Fatalf("EmailHTTPTimeout=%v", cfg.EmailHTTPTimeout)
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("EMAIL_PROVIDER", "unconfigured")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EmailProvider != config.EmailProviderUnconfigured {
		t.Fatalf("EmailProvider=%q", cfg.EmailProvider)
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("EMAIL_PROVIDER", "RESEND")
	t.Setenv("RESEND_API_KEY", "test-resend-api-key-do-not-leak")
	t.Setenv("FROM_EMAIL", "noreply@example.com")
	t.Setenv("FROM_NAME", "Haradan")
	t.Setenv("FRONTEND_URL", "https://app.example.com")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EmailProvider != config.EmailProviderResend {
		t.Fatalf("EmailProvider=%q", cfg.EmailProvider)
	}
	if cfg.FromEmail != "noreply@example.com" || cfg.FromName != "Haradan" {
		t.Fatalf("from fields: email=%q name=%q", cfg.FromEmail, cfg.FromName)
	}
	if cfg.ResendWelcomeTemplateID != "welcome-email" {
		t.Fatalf("welcome template id=%q", cfg.ResendWelcomeTemplateID)
	}
	if cfg.ResendResetPasswordTemplateID != "reset-password" {
		t.Fatalf("reset template id=%q", cfg.ResendResetPasswordTemplateID)
	}
}

func TestLoadEmailProviderResendValidation(t *testing.T) {
	setResendBase := func(t *testing.T) {
		t.Helper()
		t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
		t.Setenv("EMAIL_PROVIDER", "resend")
		t.Setenv("RESEND_API_KEY", "test-resend-api-key-do-not-leak")
		t.Setenv("FROM_EMAIL", "noreply@example.com")
		t.Setenv("FROM_NAME", "Haradan")
		t.Setenv("FRONTEND_URL", "https://app.example.com")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("EMAIL_PROVIDER", "sendgrid")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected unknown provider error")
	}

	required := []string{
		"RESEND_API_KEY",
		"FROM_EMAIL",
		"FROM_NAME",
		"FRONTEND_URL",
	}
	for _, missing := range required {
		clearConfigEnv(t)
		setResendBase(t)
		_ = os.Unsetenv(missing)
		_, err := config.Load()
		if err == nil {
			t.Fatalf("expected error when %s missing", missing)
		}
		if strings.Contains(err.Error(), "test-resend-api-key") {
			t.Fatalf("error leaked api key for missing %s: %v", missing, err)
		}
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("FROM_EMAIL", "not-an-email")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected invalid FROM_EMAIL error")
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("FROM_EMAIL", "Name <noreply@example.com>")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected display-name FROM_EMAIL rejection")
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("FROM_NAME", "Bad\nName")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected CRLF FROM_NAME rejection")
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("FROM_EMAIL", "bad\r@example.com")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected CRLF FROM_EMAIL rejection")
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("RESEND_REGISTRATION_VERIFICATION_TEMPLATE_ID", "tmpl\nid")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected CRLF template ID rejection")
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("FRONTEND_URL", "ftp://app.example.com")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected invalid FRONTEND_URL scheme error")
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("FRONTEND_URL", "https://user:pass@app.example.com")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected FRONTEND_URL userinfo rejection")
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("RESEND_BASE_URL", "http://api.resend.com")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected http base URL rejection")
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("RESEND_BASE_URL", "https://user:pass@api.resend.com")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected userinfo rejection")
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("RESEND_BASE_URL", "https://api.resend.com?x=1")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected query rejection")
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("RESEND_BASE_URL", "https://api.resend.com#frag")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected fragment rejection")
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("RESEND_BASE_URL", "https://api.resend.com/v1")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected path rejection")
	}

	clearConfigEnv(t)
	setResendBase(t)
	t.Setenv("EMAIL_HTTP_TIMEOUT", "0s")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected timeout <= 0 rejection")
	}

	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/haradan?sslmode=disable")
	t.Setenv("EMAIL_PROVIDER", "unconfigured")
	// Resend fields must not be required when provider is unconfigured.
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EmailProvider != config.EmailProviderUnconfigured {
		t.Fatalf("EmailProvider=%q", cfg.EmailProvider)
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"APP_ENV", "HTTP_ADDR", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT", "HTTP_SHUTDOWN_TIMEOUT",
		"DATABASE_URL", "DB_MAX_CONNS", "DB_MIN_CONNS", "DB_MAX_CONN_LIFETIME", "DB_MAX_CONN_IDLE_TIME", "DB_HEALTH_TIMEOUT",
		"AUTH_JWT_SECRET", "AUTH_ACCESS_TOKEN_TTL", "AUTH_REFRESH_ABSOLUTE_TTL", "AUTH_REFRESH_IDLE_TTL",
		"AUTH_EMAIL_VERIFICATION_TTL", "AUTH_ARGON2_TIME", "AUTH_ARGON2_MEMORY_KIB", "AUTH_ARGON2_THREADS", "AUTH_ARGON2_KEY_LEN",
		"MEDIA_ALLOWED_CONTENT_TYPES", "MEDIA_MAX_BYTE_SIZE", "MEDIA_UPLOAD_URL_TTL",
		"STORAGE_PROVIDER", "S3_ENDPOINT", "S3_REGION", "S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_BASE_PATH",
		"IMAGE_PROCESSOR_PROVIDER", "TINIFY_API_KEY", "TINIFY_BASE_URL", "TINIFY_HTTP_TIMEOUT",
		"MEDIA_PROFILE_DETAIL_WIDTH", "MEDIA_PROFILE_DETAIL_HEIGHT",
		"MEDIA_PROFILE_HOMEPAGE_WIDTH", "MEDIA_PROFILE_HOMEPAGE_HEIGHT",
		"MEDIA_PROFILE_SEARCH_WIDTH", "MEDIA_PROFILE_SEARCH_HEIGHT",
		"EMAIL_PROVIDER", "RESEND_API_KEY", "FROM_EMAIL", "FROM_NAME", "FRONTEND_URL",
		"RESEND_REGISTRATION_VERIFICATION_TEMPLATE_ID", "RESEND_WELCOME_TEMPLATE_ID",
		"RESEND_RESET_PASSWORD_TEMPLATE_ID", "RESEND_BASE_URL", "EMAIL_HTTP_TIMEOUT",
		"MEDIA_PUBLIC_BASE_URL",
		"NOTIFICATION_FANOUT_BATCH_SIZE", "NOTIFICATION_EMAIL_CHUNK_SIZE",
		"PACKAGE_EXPIRY_TIMEZONE", "PACKAGE_EXPIRY_SCAN_HOUR", "PACKAGE_EXPIRY_SCHEDULER_INTERVAL",
		"JOB_SCHEDULER_REFRESH_INTERVAL",
		"PACKAGE_EXPIRY_SCAN_BATCH_SIZE",
		"WORKER_CONCURRENCY", "WORKER_POLL_INTERVAL", "WORKER_LEASE_DURATION", "WORKER_JOB_TIMEOUT",
		"WORKER_ID", "WORKER_SHUTDOWN_TIMEOUT", "WORKER_RETRY_BASE_DELAY", "WORKER_RETRY_MAX_DELAY",
		"WORKER_LEASE_RECOVERY_INTERVAL",
		"TJK_ENABLED", "TJK_BASE_URL", "TJK_HTTP_TIMEOUT", "TJK_BATCH_SIZE", "TJK_MAX_BODY_BYTES",
	}
	for _, key := range keys {
		prev, had := os.LookupEnv(key)
		_ = os.Unsetenv(key)
		key, prev, had := key, prev, had
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(key, prev)
				return
			}
			_ = os.Unsetenv(key)
		})
	}
}
