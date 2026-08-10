package resendemail

import (
	"context"
	"strings"
)

const (
	maxIdempotencyKeyLen = 256
	minIdempotencyKeyLen = 1
)

// SendTemplateEmail delivers a Resend template email with an idempotency key.
func (s *Sender) SendTemplateEmail(
	ctx context.Context,
	toEmail string,
	templateID string,
	subjectFallback *string,
	variables map[string]string,
	idempotencyKey string,
) error {
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
	tmplID := strings.TrimSpace(templateID)
	if tmplID == "" {
		return dependencyError()
	}
	key := strings.TrimSpace(idempotencyKey)
	if len(key) < minIdempotencyKeyLen || len(key) > maxIdempotencyKeyLen {
		return dependencyError()
	}
	if containsCRLF(key) {
		return dependencyError()
	}

	vars := make(map[string]any, len(variables)+1)
	for k, v := range variables {
		if containsCRLF(k) || containsCRLF(v) {
			return dependencyError()
		}
		vars[k] = v
	}
	if subjectFallback != nil {
		subject := strings.TrimSpace(*subjectFallback)
		if subject != "" {
			if containsCRLF(subject) {
				return dependencyError()
			}
			vars["subject"] = subject
		}
	}
	if err := s.client.sendTemplateWithIdempotency(ctx, recipient, tmplID, vars, key); err != nil {
		return sanitizeErr(err)
	}
	return nil
}
