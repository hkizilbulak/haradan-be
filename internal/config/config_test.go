package config_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hkizilbulak/haradan-be/internal/config"
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
	if len(cfg.MediaAllowedContentTypes) != 0 || cfg.MediaMaxByteSize != 0 {
		t.Fatalf("media limits must stay unset by default: %+v", cfg)
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

func clearConfigEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"APP_ENV", "HTTP_ADDR", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT", "HTTP_SHUTDOWN_TIMEOUT",
		"DATABASE_URL", "DB_MAX_CONNS", "DB_MIN_CONNS", "DB_MAX_CONN_LIFETIME", "DB_MAX_CONN_IDLE_TIME", "DB_HEALTH_TIMEOUT",
		"AUTH_JWT_SECRET", "AUTH_ACCESS_TOKEN_TTL", "AUTH_REFRESH_ABSOLUTE_TTL", "AUTH_REFRESH_IDLE_TTL",
		"AUTH_EMAIL_VERIFICATION_TTL", "AUTH_ARGON2_TIME", "AUTH_ARGON2_MEMORY_KIB", "AUTH_ARGON2_THREADS", "AUTH_ARGON2_KEY_LEN",
		"MEDIA_ALLOWED_CONTENT_TYPES", "MEDIA_MAX_BYTE_SIZE", "MEDIA_UPLOAD_URL_TTL",
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
