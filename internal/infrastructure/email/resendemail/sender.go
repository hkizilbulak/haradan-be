package resendemail

import (
	"context"
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
)

// Sender implements appauth.EmailSender via the Resend HTTP API.
type Sender struct {
	client      *resendClient
	templateID  string
	frontendURL string
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
		},
		templateID:  strings.TrimSpace(cfg.TemplateID),
		frontendURL: strings.TrimSpace(cfg.FrontendURL),
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
		},
		templateID:  strings.TrimSpace(cfg.TemplateID),
		frontendURL: strings.TrimSpace(cfg.FrontendURL),
	}, nil
}

func formatFromHeader(name, email string) string {
	return (&mail.Address{Name: name, Address: email}).String()
}

// SendRegistrationVerification delivers the registration verification template email.
func (s *Sender) SendRegistrationVerification(ctx context.Context, toEmail, plaintextToken string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.client == nil {
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

	variables := map[string]any{
		"verificationToken": token,
		"recipientEmail":    recipient,
		"frontendUrl":       s.frontendURL,
	}
	if err := s.client.sendTemplate(ctx, recipient, s.templateID, variables); err != nil {
		return sanitizeErr(err)
	}
	return nil
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
