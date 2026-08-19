package resendemail

import (
	"context"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
)

// Sender implements appauth.EmailSender via the Resend HTTP API.
type Sender struct {
	client                  *resendClient
	welcomeTemplateID       string
	resetPasswordTemplateID string
	frontendURL             string
}

var _ appauth.EmailSender = (*Sender)(nil)

// New builds a Sender with a shared HTTP client. It performs no network I/O.
func New(cfg Config) (*Sender, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	baseURL, err := parseBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Timeout: cfg.HTTPTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			// Never auto-follow: Authorization must not travel to another origin.
			return http.ErrUseLastResponse
		},
	}

	fromEmail := strings.TrimSpace(cfg.FromEmail)
	fromName := strings.TrimSpace(cfg.FromName)
	return &Sender{
		client: &resendClient{
			apiKey:  strings.TrimSpace(cfg.APIKey),
			baseURL: baseURL,
			http:    httpClient,
			from:    formatFromHeader(fromName, fromEmail),
			logger:  cfg.Logger,
		},
		welcomeTemplateID:       cfg.resolvedWelcomeTemplateID(),
		resetPasswordTemplateID: cfg.resolvedResetPasswordTemplateID(),
		frontendURL:             strings.TrimSpace(cfg.FrontendURL),
	}, nil
}

// newWithHTTPClient is used by unit tests to inject a fake transport.
func newWithHTTPClient(cfg Config, doer httpDoer) (*Sender, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	baseURL, err := parseBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	if doer == nil {
		return nil, fmt.Errorf("http client is required")
	}
	fromEmail := strings.TrimSpace(cfg.FromEmail)
	fromName := strings.TrimSpace(cfg.FromName)
	return &Sender{
		client: &resendClient{
			apiKey:  strings.TrimSpace(cfg.APIKey),
			baseURL: baseURL,
			http:    doer,
			from:    formatFromHeader(fromName, fromEmail),
			logger:  cfg.Logger,
		},
		welcomeTemplateID:       cfg.resolvedWelcomeTemplateID(),
		resetPasswordTemplateID: cfg.resolvedResetPasswordTemplateID(),
		frontendURL:             strings.TrimSpace(cfg.FrontendURL),
	}, nil
}

func formatFromHeader(name, email string) string {
	return (&mail.Address{Name: name, Address: email}).String()
}

// SendRegistrationVerification delivers the welcome/verification template email.
// Variables: verificationUrl; fullName only when non-empty after trim. Fails
// closed when the welcome template id is unset rather than reusing the
// reset-password template.
func (s *Sender) SendRegistrationVerification(ctx context.Context, toEmail, plaintextToken, fullName string) error {
	return s.sendAuthTemplate(ctx, toEmail, plaintextToken, fullName, s.welcomeTemplateID, "verificationUrl", "/verify-email")
}

// SendPasswordReset delivers the reset-password template. Variables: resetUrl;
// fullName only when non-empty after trim. Never reuses the registration/welcome
// template.
func (s *Sender) SendPasswordReset(ctx context.Context, toEmail, plaintextToken, fullName string) error {
	return s.sendAuthTemplate(ctx, toEmail, plaintextToken, fullName, s.resetPasswordTemplateID, "resetUrl", "/reset-password")
}

func (s *Sender) sendAuthTemplate(
	ctx context.Context,
	toEmail, plaintextToken, fullName, templateID, urlVar, path string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.client == nil {
		return dependencyError()
	}
	tmplID := strings.TrimSpace(templateID)
	if tmplID == "" {
		return dependencyError()
	}

	recipient, err := normalizeRecipient(toEmail)
	if err != nil {
		return sanitizeErr(err)
	}
	token := strings.TrimSpace(plaintextToken)
	if token == "" {
		return validationToken(invalidTokenMessage)
	}
	if containsCRLF(token) {
		return validationToken(invalidTokenMessage)
	}

	link, err := buildFrontendLink(s.frontendURL, path, token)
	if err != nil {
		return dependencyError()
	}

	variables := map[string]any{
		urlVar:           link,
		"frontendUrl":    s.frontendURL,
		"recipientEmail": recipient,
	}
	if name := strings.TrimSpace(fullName); name != "" {
		variables["fullName"] = name
		// The established company templates use the legacy lower-case key.
		// Keep both spellings while Haradan-specific templates are introduced.
		variables["fullname"] = name
	}
	// Keep the raw token available for templates that still expect it.
	if urlVar == "verificationUrl" {
		variables["verificationToken"] = token
		// The existing company welcome template calls its setup link resetUrl.
		variables["resetUrl"] = link
	} else {
		variables["resetToken"] = token
	}

	if err := s.client.sendTemplate(ctx, recipient, tmplID, variables); err != nil {
		return sanitizeErr(err)
	}
	return nil
}

func buildFrontendLink(frontendURL, path, token string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	if base == "" || path == "" || token == "" {
		return "", fmt.Errorf("invalid link inputs")
	}
	u, err := url.Parse(base + path)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func normalizeRecipient(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", validationRecipient(invalidRecipientMessage)
	}
	lower := strings.ToLower(trimmed)
	if lower == "undefined" || lower == "null" {
		return "", validationRecipient(invalidRecipientMessage)
	}
	addr, err := validateMailboxAddress(trimmed, false)
	if err != nil {
		return "", validationRecipient(invalidRecipientMessage)
	}
	return addr, nil
}
