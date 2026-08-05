package tjk

import (
	"errors"
	"fmt"
	"net"
	"net/http"
)

// ErrorKind classifies TJK HTTP failures for the worker retry layer.
type ErrorKind int

const (
	// KindPermanent is not retryable (4xx except 429, redirects, oversized body, config).
	KindPermanent ErrorKind = iota
	// KindTransient is retryable (timeouts, network, 429, 5xx).
	KindTransient
)

// Error is a TJK adapter failure without raw HTML or request URLs in its message.
type Error struct {
	Kind   ErrorKind
	Status int
	msg    string
}

func (e *Error) Error() string {
	if e == nil {
		return "tjk error"
	}
	if e.Status > 0 {
		return fmt.Sprintf("%s (HTTP %d)", e.msg, e.Status)
	}
	return e.msg
}

// Retryable reports whether the worker should requeue the TJK batch job.
func (e *Error) Retryable() bool {
	return e != nil && e.Kind == KindTransient
}

func permanentErr(msg string, status int) error {
	return &Error{Kind: KindPermanent, Status: status, msg: msg}
}

func transientErr(msg string, status int) error {
	return &Error{Kind: KindTransient, Status: status, msg: msg}
}

// IsTransient reports whether err should be retried by the worker layer.
func IsTransient(err error) bool {
	var te *Error
	if errors.As(err, &te) {
		return te.Kind == KindTransient
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

// IsPermanent reports whether err must not be retried.
func IsPermanent(err error) bool {
	var te *Error
	if errors.As(err, &te) {
		return te.Kind == KindPermanent
	}
	return false
}

func mapHTTPStatus(status int) error {
	switch {
	case status == http.StatusOK:
		return nil
	case status == http.StatusTooManyRequests:
		return transientErr("TJK rate limited", status)
	case status >= 500 && status <= 599:
		return transientErr("TJK upstream unavailable", status)
	case status >= 400 && status <= 499:
		return permanentErr("TJK request rejected", status)
	case status >= 300 && status <= 399:
		return permanentErr("TJK redirect blocked", status)
	default:
		return permanentErr("TJK unexpected status", status)
	}
}

func mapTransportError(err error) error {
	if err == nil {
		return nil
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return transientErr("TJK request timed out", 0)
	}
	return transientErr("TJK network error", 0)
}
