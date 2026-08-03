package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/hkizilbulak/haradan-be/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.AppEnv != "development" {
		t.Fatalf("AppEnv=%q, want development", cfg.AppEnv)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr=%q, want :8080", cfg.HTTPAddr)
	}
	if cfg.HTTPReadTimeout != 10*time.Second {
		t.Fatalf("HTTPReadTimeout=%v", cfg.HTTPReadTimeout)
	}
	if cfg.HTTPWriteTimeout != 15*time.Second {
		t.Fatalf("HTTPWriteTimeout=%v", cfg.HTTPWriteTimeout)
	}
	if cfg.HTTPIdleTimeout != 60*time.Second {
		t.Fatalf("HTTPIdleTimeout=%v", cfg.HTTPIdleTimeout)
	}
	if cfg.HTTPShutdownTimeout != 10*time.Second {
		t.Fatalf("HTTPShutdownTimeout=%v", cfg.HTTPShutdownTimeout)
	}
}

func TestLoadValidTimeouts(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("HTTP_READ_TIMEOUT", "3s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "4s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "5s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "6s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.AppEnv != "production" || cfg.HTTPAddr != ":9090" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if cfg.HTTPReadTimeout != 3*time.Second || cfg.HTTPWriteTimeout != 4*time.Second {
		t.Fatalf("unexpected timeouts: %+v", cfg)
	}
	if cfg.HTTPIdleTimeout != 5*time.Second || cfg.HTTPShutdownTimeout != 6*time.Second {
		t.Fatalf("unexpected timeouts: %+v", cfg)
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("HTTP_READ_TIMEOUT", "not-a-duration")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestLoadEmptyHTTPAddr(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("HTTP_ADDR", "   ")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for empty HTTP_ADDR")
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"APP_ENV",
		"HTTP_ADDR",
		"HTTP_READ_TIMEOUT",
		"HTTP_WRITE_TIMEOUT",
		"HTTP_IDLE_TIMEOUT",
		"HTTP_SHUTDOWN_TIMEOUT",
	}
	for _, key := range keys {
		prev, had := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
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
