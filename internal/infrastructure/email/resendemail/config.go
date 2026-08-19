package resendemail

import (
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"
)

const (
	defaultWelcomeTemplateID       = "haradan-welcome"
	defaultResetPasswordTemplateID = "reset-password"
)

// Config holds Resend adapter settings. Values come from process configuration.
type Config struct {
	APIKey                  string
	BaseURL                 string
	HTTPTimeout             time.Duration
	FromEmail               string
	FromName                string
	FrontendURL             string
	WelcomeTemplateID       string
	ResetPasswordTemplateID string
	// TemplateID is a legacy alias for WelcomeTemplateID (registration verification).
	TemplateID string
	// Logger is optional; when set, Resend API errors are logged with their body.
	Logger interface{ Error(string, ...any) }
}

func (c Config) validate() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("resend API key must not be empty")
	}
	if containsCRLF(c.APIKey) {
		return fmt.Errorf("resend API key must not contain CR or LF")
	}
	if c.HTTPTimeout <= 0 {
		return fmt.Errorf("resend HTTP timeout must be greater than zero")
	}
	if _, err := parseBaseURL(c.BaseURL); err != nil {
		return err
	}
	fromEmail := strings.TrimSpace(c.FromEmail)
	if fromEmail == "" {
		return fmt.Errorf("from email must not be empty")
	}
	if containsCRLF(fromEmail) {
		return fmt.Errorf("from email must not contain CR or LF")
	}
	if _, err := validateMailboxAddress(fromEmail, false); err != nil {
		return fmt.Errorf("from email is not a valid email address")
	}
	fromName := strings.TrimSpace(c.FromName)
	if fromName == "" {
		return fmt.Errorf("from name must not be empty")
	}
	if containsCRLF(fromName) {
		return fmt.Errorf("from name must not contain CR or LF")
	}
	frontend := strings.TrimSpace(c.FrontendURL)
	if frontend == "" {
		return fmt.Errorf("frontend URL must not be empty")
	}
	if containsCRLF(frontend) {
		return fmt.Errorf("frontend URL must not contain CR or LF")
	}
	if err := validateFrontendURL(frontend); err != nil {
		return err
	}
	welcome := c.resolvedWelcomeTemplateID()
	if welcome == "" {
		return fmt.Errorf("welcome email template ID must not be empty")
	}
	if containsCRLF(welcome) {
		return fmt.Errorf("welcome email template ID must not contain CR or LF")
	}
	reset := c.resolvedResetPasswordTemplateID()
	if reset == "" {
		return fmt.Errorf("reset password template ID must not be empty")
	}
	if containsCRLF(reset) {
		return fmt.Errorf("reset password template ID must not contain CR or LF")
	}
	if welcome == reset {
		return fmt.Errorf("welcome and reset password template IDs must differ")
	}
	return nil
}

func (c Config) resolvedWelcomeTemplateID() string {
	if v := strings.TrimSpace(c.WelcomeTemplateID); v != "" {
		return v
	}
	if v := strings.TrimSpace(c.TemplateID); v != "" {
		return v
	}
	return defaultWelcomeTemplateID
}

func (c Config) resolvedResetPasswordTemplateID() string {
	if v := strings.TrimSpace(c.ResetPasswordTemplateID); v != "" {
		return v
	}
	return defaultResetPasswordTemplateID
}

func parseBaseURL(raw string) (*url.URL, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("resend base URL must not be empty")
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("resend base URL is not a valid URL")
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("resend base URL must use https")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("resend base URL must not contain userinfo, query, or fragment")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("resend base URL host must not be empty")
	}
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	if path != "" {
		return nil, fmt.Errorf("resend base URL must not include a path")
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host}, nil
}

func validateFrontendURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("frontend URL is not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("frontend URL must use http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("frontend URL host must not be empty")
	}
	if u.User != nil {
		return fmt.Errorf("frontend URL must not contain userinfo")
	}
	return nil
}

// validateMailboxAddress parses a single mailbox. When allowDisplayName is false,
// a display-name prefix is rejected.
func validateMailboxAddress(raw string, allowDisplayName bool) (string, error) {
	if containsCRLF(raw) {
		return "", fmt.Errorf("address must not contain CR or LF")
	}
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if addr.Address == "" || !strings.Contains(addr.Address, "@") {
		return "", fmt.Errorf("invalid address")
	}
	if !allowDisplayName && addr.Name != "" {
		return "", fmt.Errorf("display name not allowed")
	}
	return addr.Address, nil
}

func containsCRLF(s string) bool {
	return strings.ContainsAny(s, "\r\n")
}
