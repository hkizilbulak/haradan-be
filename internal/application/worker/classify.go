package worker

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

const (
	maxLastErrorLen           = 512
	safeInternalErrorMessage  = "Beklenmeyen bir hata oluştu."
	safeShutdownErrorMessage  = "İşçi kapanışı nedeniyle iş yeniden kuyruğa alındı."
	safeTimeoutErrorMessage   = "İş zaman aşımına uğradı."
	safeCanceledErrorMessage  = "İş iptal edildi."
	safeUnsupportedJobMessage = "Desteklenmeyen iş türü."
	safePayloadErrorMessage   = "Geçersiz iş yükü."
)

type outcomeKind int

const (
	outcomeSuccess outcomeKind = iota
	outcomePermanentFail
	outcomeTransientRetry
	outcomeShutdownRequeue
)

type classifiedOutcome struct {
	Kind      outcomeKind
	LastError string
}

func classifyProcessError(err error, shuttingDown bool) classifiedOutcome {
	if err == nil {
		return classifiedOutcome{Kind: outcomeSuccess}
	}
	// Hard-cancel and drain cancellation must not look like a transient timeout retry.
	if shuttingDown && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return classifiedOutcome{Kind: outcomeShutdownRequeue, LastError: safeShutdownErrorMessage}
	}
	if errors.Is(err, context.Canceled) {
		return classifiedOutcome{Kind: outcomeTransientRetry, LastError: safeCanceledErrorMessage}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return classifiedOutcome{Kind: outcomeTransientRetry, LastError: safeTimeoutErrorMessage}
	}

	ae, ok := apperr.As(err)
	if !ok {
		return classifiedOutcome{Kind: outcomeTransientRetry, LastError: safeInternalErrorMessage}
	}

	switch {
	case ae.Kind == apperr.KindValidation,
		ae.Kind == apperr.KindBadRequest,
		ae.Kind == apperr.KindNotFound,
		ae.Code == apperr.CodeInvalidState:
		return classifiedOutcome{Kind: outcomePermanentFail, LastError: sanitizeLastError(ae.Message, safePayloadErrorMessage)}
	case ae.Kind == apperr.KindDependencyUnavailable:
		return classifiedOutcome{Kind: outcomeTransientRetry, LastError: sanitizeLastError(ae.Message, safeInternalErrorMessage)}
	default:
		return classifiedOutcome{Kind: outcomeTransientRetry, LastError: sanitizeLastError(ae.Message, safeInternalErrorMessage)}
	}
}

func sanitizeLastError(msg, fallback string) string {
	s := strings.TrimSpace(msg)
	if s == "" {
		return fallback
	}
	if len(s) <= maxLastErrorLen {
		if !utf8.ValidString(s) {
			return fallback
		}
		return s
	}
	// Truncate on a UTF-8 boundary.
	trunc := s[:maxLastErrorLen]
	for len(trunc) > 0 && !utf8.ValidString(trunc) {
		trunc = trunc[:len(trunc)-1]
	}
	if trunc == "" {
		return fallback
	}
	return trunc
}
