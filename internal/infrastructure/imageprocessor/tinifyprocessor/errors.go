package tinifyprocessor

import (
	"context"
	"errors"
	"net"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

const (
	dependencyUnavailableMessage  = "Görsel işleme servisi şu anda kullanılamıyor."
	invalidImageMessage           = "Geçersiz görsel."
	unsupportedImageMessage       = "Desteklenmeyen görsel formatı."
	invalidProfileMessage         = "Geçersiz dönüşüm profili."
	processorMisconfiguredMessage = "Görsel işleme yapılandırması geçersiz."
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

func validationImage(message string, field string) error {
	if message == "" {
		message = invalidImageMessage
	}
	if field == "" {
		field = "file"
	}
	return apperr.Validation(message, apperr.FieldError{
		Field:   field,
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

func mapTinifyStatus(status int) error {
	switch {
	case status == 400 || status == 415:
		return validationImage(invalidImageMessage, "file")
	case status == 401 || status == 403:
		return dependencyError()
	case status == 429:
		return dependencyError()
	case status >= 500 && status <= 599:
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
