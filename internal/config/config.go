package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds process configuration loaded from environment variables.
type Config struct {
	AppEnv              string
	HTTPAddr            string
	HTTPReadTimeout     time.Duration
	HTTPWriteTimeout    time.Duration
	HTTPIdleTimeout     time.Duration
	HTTPShutdownTimeout time.Duration
}

// Load reads configuration from the process environment.
// It does not read .env files.
func Load() (Config, error) {
	cfg := Config{
		AppEnv: getenvDefault("APP_ENV", "development"),
	}

	if v, ok := os.LookupEnv("HTTP_ADDR"); ok {
		addr := strings.TrimSpace(v)
		if addr == "" {
			return Config{}, fmt.Errorf("HTTP_ADDR must not be empty")
		}
		cfg.HTTPAddr = addr
	} else {
		cfg.HTTPAddr = ":8080"
	}

	var err error
	if cfg.HTTPReadTimeout, err = durationEnv("HTTP_READ_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.HTTPWriteTimeout, err = durationEnv("HTTP_WRITE_TIMEOUT", 15*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.HTTPIdleTimeout, err = durationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.HTTPShutdownTimeout, err = durationEnv("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func getenvDefault(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid duration", key)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}
	return d, nil
}
