package s3storage

import (
	"context"
	"errors"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

const (
	dependencyUnavailableMessage = "Görsel depolama servisi şu anda kullanılamıyor."
	objectNotFoundMessage        = "Nesne bulunamadı."
	invalidRequestMessage        = "Geçersiz istek."
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

func isObjectNotFound(err error) bool {
	if err == nil {
		return false
	}
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return true
		case "NoSuchBucket":
			// Misconfigured bucket is a provider/config failure, not a missing object.
			return false
		default:
			// Fall through to HTTP status only when no typed object-missing code is present.
		}
	}
	// Bare HTTP 404 without a typed API code: treat as missing object for HeadObject
	// confirm semantics. NoSuchBucket is handled above when the SDK exposes it.
	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound {
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchBucket" {
			return false
		}
		return true
	}
	return false
}

func mapProviderDependency(err error) error {
	if err == nil {
		return nil
	}
	if mapped := mapContextError(err); mapped != nil {
		return mapped
	}
	return apperr.DependencyUnavailable(dependencyUnavailableMessage)
}

func mapGetObjectError(err error) error {
	if err == nil {
		return nil
	}
	if mapped := mapContextError(err); mapped != nil {
		return mapped
	}
	if isObjectNotFound(err) {
		return apperr.NotFound(objectNotFoundMessage)
	}
	return apperr.DependencyUnavailable(dependencyUnavailableMessage)
}
