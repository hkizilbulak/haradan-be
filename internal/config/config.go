package config

import (
	"fmt"
	"os"
	"strconv"
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

	DatabaseURL       string
	DBMaxConns        int32
	DBMinConns        int32
	DBMaxConnLifetime time.Duration
	DBMaxConnIdleTime time.Duration
	DBHealthTimeout   time.Duration

	// Auth settings. Token TTLs are config-injected because docs leave exact
	// durations unlocked; values here are engineering defaults, not product locks.
	AuthJWTSecret        string
	AccessTokenTTL       time.Duration
	RefreshAbsoluteTTL   time.Duration
	RefreshIdleTTL       time.Duration
	EmailVerificationTTL time.Duration
	Argon2Time           uint32
	Argon2MemoryKiB      uint32
	Argon2Threads        uint8
	Argon2KeyLen         uint32

	// Media upload settings. All three are optional: the exact MIME allowlist,
	// byte ceiling and pixel bounds are open product/technical decisions, so no
	// value is defaulted here. While the allowlist is empty or the byte ceiling
	// is unset, upload initiation reports the dependency as unavailable instead
	// of accepting a file under invented limits. No storage or compression
	// provider credentials are read here.
	MediaAllowedContentTypes []string
	MediaMaxByteSize         int64
	MediaUploadURLTTL        time.Duration
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

	cfg.DatabaseURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL must not be empty")
	}

	if cfg.DBMaxConns, err = int32Env("DB_MAX_CONNS", 10); err != nil {
		return Config{}, err
	}
	if cfg.DBMinConns, err = int32Env("DB_MIN_CONNS", 2); err != nil {
		return Config{}, err
	}
	if cfg.DBMaxConns < 1 {
		return Config{}, fmt.Errorf("DB_MAX_CONNS must be at least 1")
	}
	if cfg.DBMinConns < 0 {
		return Config{}, fmt.Errorf("DB_MIN_CONNS must not be negative")
	}
	if cfg.DBMinConns > cfg.DBMaxConns {
		return Config{}, fmt.Errorf("DB_MIN_CONNS must not be greater than DB_MAX_CONNS")
	}

	if cfg.DBMaxConnLifetime, err = durationEnv("DB_MAX_CONN_LIFETIME", 30*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.DBMaxConnIdleTime, err = durationEnv("DB_MAX_CONN_IDLE_TIME", 5*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.DBHealthTimeout, err = durationEnv("DB_HEALTH_TIMEOUT", 2*time.Second); err != nil {
		return Config{}, err
	}

	cfg.AuthJWTSecret = strings.TrimSpace(os.Getenv("AUTH_JWT_SECRET"))
	if isProductionLike(cfg.AppEnv) && cfg.AuthJWTSecret == "" {
		return Config{}, fmt.Errorf("AUTH_JWT_SECRET must not be empty in %s", cfg.AppEnv)
	}

	if cfg.AccessTokenTTL, err = durationEnv("AUTH_ACCESS_TOKEN_TTL", 15*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.RefreshAbsoluteTTL, err = durationEnv("AUTH_REFRESH_ABSOLUTE_TTL", 30*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.RefreshIdleTTL, err = durationEnv("AUTH_REFRESH_IDLE_TTL", 7*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.EmailVerificationTTL, err = durationEnv("AUTH_EMAIL_VERIFICATION_TTL", 24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.RefreshIdleTTL > cfg.RefreshAbsoluteTTL {
		return Config{}, fmt.Errorf("AUTH_REFRESH_IDLE_TTL must not exceed AUTH_REFRESH_ABSOLUTE_TTL")
	}

	if cfg.Argon2Time, err = uint32Env("AUTH_ARGON2_TIME", 3); err != nil {
		return Config{}, err
	}
	if cfg.Argon2MemoryKiB, err = uint32Env("AUTH_ARGON2_MEMORY_KIB", 64*1024); err != nil {
		return Config{}, err
	}
	if threads, err := uint32Env("AUTH_ARGON2_THREADS", 2); err != nil {
		return Config{}, err
	} else {
		if threads == 0 || threads > 255 {
			return Config{}, fmt.Errorf("AUTH_ARGON2_THREADS out of range")
		}
		cfg.Argon2Threads = uint8(threads)
	}
	if cfg.Argon2KeyLen, err = uint32Env("AUTH_ARGON2_KEY_LEN", 32); err != nil {
		return Config{}, err
	}
	if cfg.Argon2Time == 0 || cfg.Argon2MemoryKiB == 0 || cfg.Argon2KeyLen == 0 {
		return Config{}, fmt.Errorf("argon2 parameters must be greater than zero")
	}

	cfg.MediaAllowedContentTypes = csvEnv("MEDIA_ALLOWED_CONTENT_TYPES")
	if cfg.MediaMaxByteSize, err = int64Env("MEDIA_MAX_BYTE_SIZE", 0); err != nil {
		return Config{}, err
	}
	if cfg.MediaMaxByteSize < 0 {
		return Config{}, fmt.Errorf("MEDIA_MAX_BYTE_SIZE must not be negative")
	}
	if cfg.MediaUploadURLTTL, err = durationEnv("MEDIA_UPLOAD_URL_TTL", 15*time.Minute); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func isProductionLike(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "production", "staging", "prod":
		return true
	default:
		return false
	}
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

func int32Env(key string, fallback int32) (int32, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid integer", key)
	}
	return int32(n), nil
}

func int64Env(key string, fallback int64) (int64, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid integer", key)
	}
	return n, nil
}

// csvEnv reads a comma-separated list, dropping blank entries. An unset or empty
// variable yields nil so callers can tell "not configured" from "configured".
func csvEnv(key string) []string {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func uint32Env(key string, fallback uint32) (uint32, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	n, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid integer", key)
	}
	return uint32(n), nil
}
