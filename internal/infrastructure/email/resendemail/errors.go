package resendemail

import (
	"context"
	"errors"
	"net"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

const (
	dependencyUnavailableMessage = "E-posta servisi şu anda kullanılamıyor."
	invalidRecipientMessage      = "Geçersiz e-posta adresi."
	invalidTokenMessage          = "Doğrulama jetonu geçersiz."
)

func mapContextError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
}

func validationRecipient(message string) error {
	if message == "" {
		message = invalidRecipientMessage
	}
	return apperr.Validation(message, apperr.FieldError{
		Field:   "email",
		Message: message,
	})
}

func validationToken(message string) error {
	if message == "" {
		message = invalidTokenMessage
	}
	return apperr.Validation(message, apperr.FieldError{
		Field:   "token",
		Message: message,
	})
}

func dependencyError() error {
	return apperr.DependencyUnavailable(dependencyUnavailableMessage)
}

func mapTransportError(err error) error {
	if err == nil {
		return nil
	}
	if mapped := mapContextError(err); mapped != nil {
		return mapped
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return dependencyError()
	}
	return dependencyError()
}

func mapResendStatus(status int) error {
	switch {
	case status == 401 || status == 403:
		return dependencyError()
	case status == 408:
		return dependencyError()
	case status == 429:
		return dependencyError()
	case status >= 500 && status <= 599:
		return dependencyError()
	case status >= 400 && status <= 499:
		// Provider request/config failures must not expose raw bodies.
		return dependencyError()
	default:
		return dependencyError()
	}
}

func sanitizeErr(err error) error {
	if err == nil {
		return nil
	}
	if mapped := mapContextError(err); mapped != nil {
		return mapped
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	return dependencyError()
}
