package apperr

import (
	"errors"
	"fmt"
)

// Code is a DomainErrorCode value from the OpenAPI contract.
type Code string

const (
	CodeValidation            Code = "VALIDATION_ERROR"
	CodeNotFound              Code = "NOT_FOUND"
	CodeInvalidState          Code = "INVALID_STATE"
	CodeConflict              Code = "CONFLICT"
	CodeInternal              Code = "INTERNAL_ERROR"
	CodeUnauthenticated       Code = "UNAUTHENTICATED"
	CodeForbidden             Code = "FORBIDDEN"
	CodeAccountInactive       Code = "ACCOUNT_INACTIVE"
	CodeEmailNotVerified      Code = "EMAIL_NOT_VERIFIED"
	CodeTokenInvalid          Code = "TOKEN_INVALID"
	CodeTokenExpired          Code = "TOKEN_EXPIRED"
	CodeTokenAlreadyUsed      Code = "TOKEN_ALREADY_USED"
	CodeSessionRevoked        Code = "SESSION_REVOKED"
	CodeRefreshReplayDetected Code = "REFRESH_REPLAY_DETECTED"
	CodeOwnershipRequired     Code = "OWNERSHIP_REQUIRED"
	CodeDependencyUnavailable Code = "DEPENDENCY_UNAVAILABLE"
)

// Kind classifies application/domain failures for transport mapping.
type Kind int

const (
	KindValidation Kind = iota + 1
	KindNotFound
	KindConflict
	KindInternal
	KindUnauthenticated
	KindForbidden
	KindDependencyUnavailable
	KindBadRequest
)

// FieldError describes a single field validation failure.
type FieldError struct {
	Field   string
	Message string
}

// Error is a typed domain/application error without credential details.
type Error struct {
	Kind        Kind
	Code        Code
	Message     string
	FieldErrors []FieldError
	cause       error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Validation returns a validation error.
func Validation(message string, fields ...FieldError) *Error {
	return &Error{
		Kind:        KindValidation,
		Code:        CodeValidation,
		Message:     message,
		FieldErrors: fields,
	}
}

// NotFound returns a not-found error.
func NotFound(message string) *Error {
	return &Error{Kind: KindNotFound, Code: CodeNotFound, Message: message}
}

// InvalidState returns a conflict-class invalid state error.
func InvalidState(message string) *Error {
	return &Error{Kind: KindConflict, Code: CodeInvalidState, Message: message}
}

// Conflict returns a generic conflict error.
func Conflict(message string) *Error {
	return &Error{Kind: KindConflict, Code: CodeConflict, Message: message}
}

// Unauthenticated returns a 401-class auth failure.
func Unauthenticated(code Code, message string) *Error {
	if code == "" {
		code = CodeUnauthenticated
	}
	return &Error{Kind: KindUnauthenticated, Code: code, Message: message}
}

// Forbidden returns a 403-class authorization failure.
func Forbidden(code Code, message string) *Error {
	if code == "" {
		code = CodeForbidden
	}
	return &Error{Kind: KindForbidden, Code: code, Message: message}
}

// DependencyUnavailable returns a 503-class dependency failure.
func DependencyUnavailable(message string) *Error {
	return &Error{
		Kind:    KindDependencyUnavailable,
		Code:    CodeDependencyUnavailable,
		Message: message,
	}
}

// BadRequest returns a 400-class client error with an exact DomainErrorCode.
func BadRequest(code Code, message string) *Error {
	return &Error{Kind: KindBadRequest, Code: code, Message: message}
}

// TokenInvalid returns TOKEN_INVALID (AUTH-02).
func TokenInvalid(message string) *Error {
	return BadRequest(CodeTokenInvalid, message)
}

// TokenExpired returns TOKEN_EXPIRED (AUTH-02).
func TokenExpired(message string) *Error {
	return BadRequest(CodeTokenExpired, message)
}

// TokenAlreadyUsed returns TOKEN_ALREADY_USED (AUTH-02 / 409).
func TokenAlreadyUsed(message string) *Error {
	return &Error{Kind: KindConflict, Code: CodeTokenAlreadyUsed, Message: message}
}

// OwnershipRequired returns OWNERSHIP_REQUIRED (403).
func OwnershipRequired(message string) *Error {
	return &Error{Kind: KindForbidden, Code: CodeOwnershipRequired, Message: message}
}

// Internal wraps an unexpected failure for safe client mapping.
func Internal(cause error) *Error {
	return &Error{
		Kind:    KindInternal,
		Code:    CodeInternal,
		Message: "Beklenmeyen bir hata oluştu.",
		cause:   cause,
	}
}

// As extracts *Error from an error chain.
func As(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

// WrapInternal ensures non-domain errors become internal errors.
func WrapInternal(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := As(err); ok {
		return err
	}
	return Internal(fmt.Errorf("unexpected: %w", err))
}
