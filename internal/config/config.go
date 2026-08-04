package config

import (
	"fmt"
	"net/mail"
	"net/url"
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
	// of accepting a file under invented limits.
	MediaAllowedContentTypes []string
	MediaMaxByteSize         int64
	MediaUploadURLTTL        time.Duration

	// Object storage. STORAGE_PROVIDER empty/unconfigured keeps the process on
	// UnconfiguredStorage without requiring credentials. Provider "b2" requires
	// every S3_* field below (except optional S3_BASE_PATH).
	StorageProvider string
	S3Endpoint      string
	S3Region        string
	S3Bucket        string
	S3AccessKey     string
	S3SecretKey     string
	S3BasePath      string

	// Image processor. Empty/unconfigured keeps UnconfiguredImageProcessor.
	// Provider "tinify" requires API key and all profile width/height values.
	ImageProcessorProvider string
	TinifyAPIKey           string
	TinifyBaseURL          string
	TinifyHTTPTimeout      time.Duration
	MediaProfileDetailW    int
	MediaProfileDetailH    int
	MediaProfileHomepageW  int
	MediaProfileHomepageH  int
	MediaProfileSearchW    int
	MediaProfileSearchH    int

	// Email delivery. EMAIL_PROVIDER empty/unconfigured keeps NoopEmailSender
	// without requiring Resend credentials. Provider "resend" requires every
	// Resend field below.
	EmailProvider                            string
	ResendAPIKey                             string
	FromEmail                                string
	FromName                                 string
	FrontendURL                              string
	ResendRegistrationVerificationTemplateID string
	EmailHTTPTimeout                         time.Duration
	ResendBaseURL                            string

	// Background worker runtime (used by cmd/worker; API ignores these).
	WorkerConcurrency           int
	WorkerPollInterval          time.Duration
	WorkerLeaseDuration         time.Duration
	WorkerJobTimeout            time.Duration
	WorkerID                    string
	WorkerShutdownTimeout       time.Duration
	WorkerRetryBaseDelay        time.Duration
	WorkerRetryMaxDelay         time.Duration
	WorkerLeaseRecoveryInterval time.Duration
}

// Supported STORAGE_PROVIDER values after normalization.
const (
	StorageProviderUnconfigured = "unconfigured"
	StorageProviderB2           = "b2"
)

// Supported IMAGE_PROCESSOR_PROVIDER values after normalization.
const (
	ImageProcessorProviderUnconfigured = "unconfigured"
	ImageProcessorProviderTinify       = "tinify"
)

// Supported EMAIL_PROVIDER values after normalization.
const (
	EmailProviderUnconfigured = "unconfigured"
	EmailProviderResend       = "resend"
)

const defaultTinifyBaseURL = "https://api.tinify.com"
const defaultResendBaseURL = "https://api.resend.com"

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

	if cfg.StorageProvider, err = normalizeStorageProvider(os.Getenv("STORAGE_PROVIDER")); err != nil {
		return Config{}, err
	}
	cfg.S3Endpoint = strings.TrimSpace(os.Getenv("S3_ENDPOINT"))
	cfg.S3Region = strings.TrimSpace(os.Getenv("S3_REGION"))
	cfg.S3Bucket = strings.TrimSpace(os.Getenv("S3_BUCKET"))
	cfg.S3AccessKey = strings.TrimSpace(os.Getenv("S3_ACCESS_KEY"))
	cfg.S3SecretKey = strings.TrimSpace(os.Getenv("S3_SECRET_KEY"))
	if cfg.S3BasePath, err = normalizeS3BasePath(os.Getenv("S3_BASE_PATH")); err != nil {
		return Config{}, err
	}
	if err := validateStorageConfig(cfg); err != nil {
		return Config{}, err
	}

	if cfg.ImageProcessorProvider, err = normalizeImageProcessorProvider(os.Getenv("IMAGE_PROCESSOR_PROVIDER")); err != nil {
		return Config{}, err
	}
	cfg.TinifyAPIKey = strings.TrimSpace(os.Getenv("TINIFY_API_KEY"))
	if cfg.TinifyBaseURL, err = normalizeTinifyBaseURL(
		getenvDefault("TINIFY_BASE_URL", defaultTinifyBaseURL),
		cfg.AppEnv,
	); err != nil {
		return Config{}, err
	}
	if cfg.TinifyHTTPTimeout, err = durationEnv("TINIFY_HTTP_TIMEOUT", 30*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.MediaProfileDetailW, err = optionalPositiveIntEnv("MEDIA_PROFILE_DETAIL_WIDTH"); err != nil {
		return Config{}, err
	}
	if cfg.MediaProfileDetailH, err = optionalPositiveIntEnv("MEDIA_PROFILE_DETAIL_HEIGHT"); err != nil {
		return Config{}, err
	}
	if cfg.MediaProfileHomepageW, err = optionalPositiveIntEnv("MEDIA_PROFILE_HOMEPAGE_WIDTH"); err != nil {
		return Config{}, err
	}
	if cfg.MediaProfileHomepageH, err = optionalPositiveIntEnv("MEDIA_PROFILE_HOMEPAGE_HEIGHT"); err != nil {
		return Config{}, err
	}
	if cfg.MediaProfileSearchW, err = optionalPositiveIntEnv("MEDIA_PROFILE_SEARCH_WIDTH"); err != nil {
		return Config{}, err
	}
	if cfg.MediaProfileSearchH, err = optionalPositiveIntEnv("MEDIA_PROFILE_SEARCH_HEIGHT"); err != nil {
		return Config{}, err
	}
	if err := validateImageProcessorConfig(cfg); err != nil {
		return Config{}, err
	}

	if cfg.EmailProvider, err = normalizeEmailProvider(os.Getenv("EMAIL_PROVIDER")); err != nil {
		return Config{}, err
	}
	cfg.ResendAPIKey = strings.TrimSpace(os.Getenv("RESEND_API_KEY"))
	cfg.FromEmail = strings.TrimSpace(os.Getenv("FROM_EMAIL"))
	cfg.FromName = strings.TrimSpace(os.Getenv("FROM_NAME"))
	cfg.FrontendURL = strings.TrimSpace(os.Getenv("FRONTEND_URL"))
	cfg.ResendRegistrationVerificationTemplateID = strings.TrimSpace(os.Getenv("RESEND_REGISTRATION_VERIFICATION_TEMPLATE_ID"))
	if cfg.ResendBaseURL, err = normalizeResendBaseURL(
		getenvDefault("RESEND_BASE_URL", defaultResendBaseURL),
	); err != nil {
		return Config{}, err
	}
	if cfg.EmailHTTPTimeout, err = durationEnv("EMAIL_HTTP_TIMEOUT", 30*time.Second); err != nil {
		return Config{}, err
	}
	if err := validateEmailConfig(cfg); err != nil {
		return Config{}, err
	}

	if cfg.WorkerConcurrency, err = intEnv("WORKER_CONCURRENCY", 2); err != nil {
		return Config{}, err
	}
	if cfg.WorkerPollInterval, err = durationEnv("WORKER_POLL_INTERVAL", time.Second); err != nil {
		return Config{}, err
	}
	if cfg.WorkerLeaseDuration, err = durationEnv("WORKER_LEASE_DURATION", 2*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.WorkerJobTimeout, err = durationEnv("WORKER_JOB_TIMEOUT", 60*time.Second); err != nil {
		return Config{}, err
	}
	if v, ok := os.LookupEnv("WORKER_ID"); ok {
		id := strings.TrimSpace(v)
		if id == "" {
			return Config{}, fmt.Errorf("WORKER_ID must not be empty when set")
		}
		cfg.WorkerID = id
	}
	if cfg.WorkerShutdownTimeout, err = durationEnv("WORKER_SHUTDOWN_TIMEOUT", 30*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.WorkerRetryBaseDelay, err = durationEnv("WORKER_RETRY_BASE_DELAY", 5*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.WorkerRetryMaxDelay, err = durationEnv("WORKER_RETRY_MAX_DELAY", 5*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.WorkerLeaseRecoveryInterval, err = durationEnv("WORKER_LEASE_RECOVERY_INTERVAL", 30*time.Second); err != nil {
		return Config{}, err
	}
	if err := validateWorkerConfig(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func normalizeStorageProvider(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", StorageProviderUnconfigured:
		return StorageProviderUnconfigured, nil
	case StorageProviderB2, "backblaze":
		return StorageProviderB2, nil
	default:
		return "", fmt.Errorf("STORAGE_PROVIDER is not supported")
	}
}

func normalizeS3BasePath(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if strings.Contains(s, `\`) {
		return "", fmt.Errorf("S3_BASE_PATH must not contain backslashes")
	}
	s = strings.Trim(s, "/")
	if s == "" {
		return "", nil
	}
	parts := strings.Split(s, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("S3_BASE_PATH contains an invalid path segment")
		}
		out = append(out, part)
	}
	return strings.Join(out, "/"), nil
}

func validateStorageConfig(cfg Config) error {
	switch cfg.StorageProvider {
	case StorageProviderUnconfigured:
		return nil
	case StorageProviderB2:
		if cfg.S3Endpoint == "" {
			return fmt.Errorf("S3_ENDPOINT must not be empty when STORAGE_PROVIDER=b2")
		}
		if cfg.S3Region == "" {
			return fmt.Errorf("S3_REGION must not be empty when STORAGE_PROVIDER=b2")
		}
		if cfg.S3Bucket == "" {
			return fmt.Errorf("S3_BUCKET must not be empty when STORAGE_PROVIDER=b2")
		}
		if cfg.S3AccessKey == "" {
			return fmt.Errorf("S3_ACCESS_KEY must not be empty when STORAGE_PROVIDER=b2")
		}
		if cfg.S3SecretKey == "" {
			return fmt.Errorf("S3_SECRET_KEY must not be empty when STORAGE_PROVIDER=b2")
		}
		return nil
	default:
		return fmt.Errorf("STORAGE_PROVIDER is not supported")
	}
}

func normalizeImageProcessorProvider(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", ImageProcessorProviderUnconfigured:
		return ImageProcessorProviderUnconfigured, nil
	case ImageProcessorProviderTinify:
		return ImageProcessorProviderTinify, nil
	default:
		return "", fmt.Errorf("IMAGE_PROCESSOR_PROVIDER is not supported")
	}
}

func normalizeTinifyBaseURL(raw, appEnv string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		s = defaultTinifyBaseURL
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("TINIFY_BASE_URL is not a valid URL")
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("TINIFY_BASE_URL must use https")
	}
	if isProductionLike(appEnv) && u.Scheme != "https" {
		return "", fmt.Errorf("TINIFY_BASE_URL must use https in %s", appEnv)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("TINIFY_BASE_URL must not contain userinfo, query, or fragment")
	}
	if u.Host == "" {
		return "", fmt.Errorf("TINIFY_BASE_URL host must not be empty")
	}
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	if path != "" {
		return "", fmt.Errorf("TINIFY_BASE_URL must not include a path")
	}
	return strings.TrimRight(u.Scheme+"://"+u.Host, "/"), nil
}

func validateImageProcessorConfig(cfg Config) error {
	switch cfg.ImageProcessorProvider {
	case ImageProcessorProviderUnconfigured:
		return nil
	case ImageProcessorProviderTinify:
		if cfg.TinifyAPIKey == "" {
			return fmt.Errorf("TINIFY_API_KEY must not be empty when IMAGE_PROCESSOR_PROVIDER=tinify")
		}
		if cfg.TinifyBaseURL == "" {
			return fmt.Errorf("TINIFY_BASE_URL must not be empty when IMAGE_PROCESSOR_PROVIDER=tinify")
		}
		if cfg.TinifyHTTPTimeout <= 0 {
			return fmt.Errorf("TINIFY_HTTP_TIMEOUT must be greater than zero")
		}
		dims := []struct {
			name string
			v    int
		}{
			{"MEDIA_PROFILE_DETAIL_WIDTH", cfg.MediaProfileDetailW},
			{"MEDIA_PROFILE_DETAIL_HEIGHT", cfg.MediaProfileDetailH},
			{"MEDIA_PROFILE_HOMEPAGE_WIDTH", cfg.MediaProfileHomepageW},
			{"MEDIA_PROFILE_HOMEPAGE_HEIGHT", cfg.MediaProfileHomepageH},
			{"MEDIA_PROFILE_SEARCH_WIDTH", cfg.MediaProfileSearchW},
			{"MEDIA_PROFILE_SEARCH_HEIGHT", cfg.MediaProfileSearchH},
		}
		for _, d := range dims {
			if d.v <= 0 {
				return fmt.Errorf("%s must be greater than zero when IMAGE_PROCESSOR_PROVIDER=tinify", d.name)
			}
		}
		for _, ct := range cfg.MediaAllowedContentTypes {
			switch strings.ToLower(strings.TrimSpace(ct)) {
			case "image/jpeg", "image/png":
				continue
			default:
				return fmt.Errorf("MEDIA_ALLOWED_CONTENT_TYPES may only include image/jpeg and image/png when IMAGE_PROCESSOR_PROVIDER=tinify")
			}
		}
		return nil
	default:
		return fmt.Errorf("IMAGE_PROCESSOR_PROVIDER is not supported")
	}
}

func normalizeEmailProvider(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", EmailProviderUnconfigured:
		return EmailProviderUnconfigured, nil
	case EmailProviderResend:
		return EmailProviderResend, nil
	default:
		return "", fmt.Errorf("EMAIL_PROVIDER is not supported")
	}
}

func normalizeResendBaseURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		s = defaultResendBaseURL
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("RESEND_BASE_URL is not a valid URL")
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("RESEND_BASE_URL must use https")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("RESEND_BASE_URL must not contain userinfo, query, or fragment")
	}
	if u.Host == "" {
		return "", fmt.Errorf("RESEND_BASE_URL host must not be empty")
	}
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	if path != "" {
		return "", fmt.Errorf("RESEND_BASE_URL must not include a path")
	}
	return strings.TrimRight(u.Scheme+"://"+u.Host, "/"), nil
}

func validateEmailConfig(cfg Config) error {
	if cfg.EmailHTTPTimeout <= 0 {
		return fmt.Errorf("EMAIL_HTTP_TIMEOUT must be greater than zero")
	}
	if cfg.ResendBaseURL == "" {
		return fmt.Errorf("RESEND_BASE_URL must not be empty")
	}

	switch cfg.EmailProvider {
	case EmailProviderUnconfigured:
		return nil
	case EmailProviderResend:
		if cfg.ResendAPIKey == "" {
			return fmt.Errorf("RESEND_API_KEY must not be empty when EMAIL_PROVIDER=resend")
		}
		if containsCRLF(cfg.ResendAPIKey) {
			return fmt.Errorf("RESEND_API_KEY must not contain CR or LF")
		}
		if cfg.FromEmail == "" {
			return fmt.Errorf("FROM_EMAIL must not be empty when EMAIL_PROVIDER=resend")
		}
		if containsCRLF(cfg.FromEmail) {
			return fmt.Errorf("FROM_EMAIL must not contain CR or LF")
		}
		if err := validateConfigEmailAddress(cfg.FromEmail); err != nil {
			return fmt.Errorf("FROM_EMAIL is not a valid email address")
		}
		if cfg.FromName == "" {
			return fmt.Errorf("FROM_NAME must not be empty when EMAIL_PROVIDER=resend")
		}
		if containsCRLF(cfg.FromName) {
			return fmt.Errorf("FROM_NAME must not contain CR or LF")
		}
		if cfg.FrontendURL == "" {
			return fmt.Errorf("FRONTEND_URL must not be empty when EMAIL_PROVIDER=resend")
		}
		if containsCRLF(cfg.FrontendURL) {
			return fmt.Errorf("FRONTEND_URL must not contain CR or LF")
		}
		if err := validateFrontendURL(cfg.FrontendURL); err != nil {
			return err
		}
		if cfg.ResendRegistrationVerificationTemplateID == "" {
			return fmt.Errorf("RESEND_REGISTRATION_VERIFICATION_TEMPLATE_ID must not be empty when EMAIL_PROVIDER=resend")
		}
		if containsCRLF(cfg.ResendRegistrationVerificationTemplateID) {
			return fmt.Errorf("RESEND_REGISTRATION_VERIFICATION_TEMPLATE_ID must not contain CR or LF")
		}
		return nil
	default:
		return fmt.Errorf("EMAIL_PROVIDER is not supported")
	}
}

func containsCRLF(s string) bool {
	return strings.ContainsAny(s, "\r\n")
}

func validateConfigEmailAddress(raw string) error {
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return err
	}
	if addr.Name != "" {
		return fmt.Errorf("display name not allowed")
	}
	if addr.Address == "" || !strings.Contains(addr.Address, "@") {
		return fmt.Errorf("invalid address")
	}
	return nil
}

func validateFrontendURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("FRONTEND_URL is not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("FRONTEND_URL must use http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("FRONTEND_URL host must not be empty")
	}
	if u.User != nil {
		return fmt.Errorf("FRONTEND_URL must not contain userinfo")
	}
	return nil
}

func optionalPositiveIntEnv(key string) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid integer", key)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}
	return n, nil
}

func isProductionLike(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "production", "staging", "prod":
		return true
	default:
		return false
	}
}

func validateWorkerConfig(cfg Config) error {
	if cfg.WorkerConcurrency <= 0 {
		return fmt.Errorf("WORKER_CONCURRENCY must be greater than zero")
	}
	if cfg.WorkerPollInterval <= 0 {
		return fmt.Errorf("WORKER_POLL_INTERVAL must be greater than zero")
	}
	if cfg.WorkerLeaseDuration <= 0 {
		return fmt.Errorf("WORKER_LEASE_DURATION must be greater than zero")
	}
	if cfg.WorkerJobTimeout <= 0 {
		return fmt.Errorf("WORKER_JOB_TIMEOUT must be greater than zero")
	}
	if cfg.WorkerJobTimeout >= cfg.WorkerLeaseDuration {
		return fmt.Errorf("WORKER_JOB_TIMEOUT must be less than WORKER_LEASE_DURATION")
	}
	if cfg.WorkerShutdownTimeout <= 0 {
		return fmt.Errorf("WORKER_SHUTDOWN_TIMEOUT must be greater than zero")
	}
	if cfg.WorkerRetryBaseDelay <= 0 {
		return fmt.Errorf("WORKER_RETRY_BASE_DELAY must be greater than zero")
	}
	if cfg.WorkerRetryMaxDelay < cfg.WorkerRetryBaseDelay {
		return fmt.Errorf("WORKER_RETRY_BASE_DELAY must be less than or equal to WORKER_RETRY_MAX_DELAY")
	}
	if cfg.WorkerLeaseRecoveryInterval <= 0 {
		return fmt.Errorf("WORKER_LEASE_RECOVERY_INTERVAL must be greater than zero")
	}
	return nil
}

func intEnv(key string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid integer", key)
	}
	return n, nil
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
