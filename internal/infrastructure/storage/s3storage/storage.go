package s3storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

// objectAPI is the subset of the S3 client used by this adapter.
type objectAPI interface {
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// presignAPI is the subset of the S3 presign client used by this adapter.
type presignAPI interface {
	PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4PresignedHTTPRequest, error)
}

// v4PresignedHTTPRequest is the shape returned by the AWS v2 presigner. The
// local alias keeps tests free of importing the smithy signing package solely
// for the URL and method fields.
type v4PresignedHTTPRequest struct {
	URL    string
	Method string
}

type awsPresignClient struct {
	inner *s3.PresignClient
}

func (c awsPresignClient) PresignPutObject(
	ctx context.Context,
	params *s3.PutObjectInput,
	optFns ...func(*s3.PresignOptions),
) (*v4PresignedHTTPRequest, error) {
	out, err := c.inner.PresignPutObject(ctx, params, optFns...)
	if err != nil {
		return nil, err
	}
	return &v4PresignedHTTPRequest{URL: out.URL, Method: out.Method}, nil
}

// Storage is an S3-compatible (Backblaze B2) implementation of appmedia.Storage.
type Storage struct {
	bucket   string
	basePath string
	client   objectAPI
	presign  presignAPI
	now      func() time.Time
}

// New builds a Storage adapter with a single shared S3 client and presigner.
// It performs no network I/O and does not probe bucket existence.
func New(cfg Config) (*Storage, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(strings.TrimSpace(cfg.Region)),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				strings.TrimSpace(cfg.AccessKey),
				strings.TrimSpace(cfg.SecretKey),
				"",
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &Storage{
		bucket:   strings.TrimSpace(cfg.Bucket),
		basePath: cfg.BasePath,
		client:   client,
		presign:  awsPresignClient{inner: s3.NewPresignClient(client)},
		now:      func() time.Time { return time.Now().UTC() },
	}, nil
}

// newWithDeps constructs a Storage with injected clients for unit tests.
func newWithDeps(cfg Config, client objectAPI, presign presignAPI, now func() time.Time) (*Storage, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if client == nil || presign == nil {
		return nil, fmt.Errorf("storage client dependencies are required")
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Storage{
		bucket:   strings.TrimSpace(cfg.Bucket),
		basePath: cfg.BasePath,
		client:   client,
		presign:  presign,
		now:      now,
	}, nil
}

// CreateUploadAuthorization implements appmedia.Storage.
func (s *Storage) CreateUploadAuthorization(
	ctx context.Context,
	objectKey string,
	contentType string,
	maxBytes int64,
	ttl time.Duration,
) (appmedia.UploadAuth, error) {
	if ttl <= 0 {
		return appmedia.UploadAuth{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field:   "ttl",
			Message: "Yükleme süresi geçersiz.",
		})
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return appmedia.UploadAuth{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field:   "contentType",
			Message: "İçerik türü zorunludur.",
		})
	}
	if maxBytes <= 0 {
		return appmedia.UploadAuth{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field:   "maxBytes",
			Message: "Maksimum dosya boyutu geçersiz.",
		})
	}

	providerKey, err := s.providerKey(objectKey)
	if err != nil {
		return appmedia.UploadAuth{}, err
	}

	expiresAt := s.now().Add(ttl)
	out, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(providerKey),
		ContentType: aws.String(contentType),
	}, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	if err != nil {
		return appmedia.UploadAuth{}, mapProviderDependency(err)
	}

	return appmedia.UploadAuth{
		Method:    "PUT",
		URL:       out.URL,
		ExpiresAt: expiresAt,
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		ObjectKey: objectKey,
	}, nil
}

// HeadObject implements appmedia.Storage.
func (s *Storage) HeadObject(ctx context.Context, objectKey string) (appmedia.ObjectInfo, error) {
	providerKey, err := s.providerKey(objectKey)
	if err != nil {
		return appmedia.ObjectInfo{}, err
	}

	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(providerKey),
	})
	if err != nil {
		if mapped := mapContextError(err); mapped != nil {
			return appmedia.ObjectInfo{}, mapped
		}
		if isObjectNotFound(err) {
			return appmedia.ObjectInfo{Exists: false}, nil
		}
		return appmedia.ObjectInfo{}, mapProviderDependency(err)
	}

	info := appmedia.ObjectInfo{Exists: true}
	if out.ContentType != nil {
		info.ContentType = *out.ContentType
	}
	if out.ContentLength != nil {
		info.ByteSize = *out.ContentLength
	}
	return info, nil
}

// PutObject implements appmedia.Storage.
func (s *Storage) PutObject(ctx context.Context, objectKey string, contentType string, body []byte) error {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field:   "contentType",
			Message: "İçerik türü zorunludur.",
		})
	}
	providerKey, err := s.providerKey(objectKey)
	if err != nil {
		return err
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(providerKey),
		Body:          bytes.NewReader(body),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(int64(len(body))),
	})
	if err != nil {
		return mapProviderDependency(err)
	}
	return nil
}

// GetObject implements appmedia.Storage.
func (s *Storage) GetObject(ctx context.Context, objectKey string) ([]byte, string, error) {
	providerKey, err := s.providerKey(objectKey)
	if err != nil {
		return nil, "", err
	}

	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(providerKey),
	})
	if err != nil {
		return nil, "", mapGetObjectError(err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		if mapped := mapContextError(err); mapped != nil {
			return nil, "", mapped
		}
		return nil, "", apperr.Internal(fmt.Errorf("read object body: %w", err))
	}

	contentType := ""
	if out.ContentType != nil {
		contentType = *out.ContentType
	}
	return data, contentType, nil
}

// DeleteObject implements idempotent object removal.
func (s *Storage) DeleteObject(ctx context.Context, objectKey string) error {
	providerKey, err := s.providerKey(objectKey)
	if err != nil {
		return err
	}
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(providerKey)})
	if err == nil || isObjectNotFound(err) {
		return nil
	}
	return mapProviderDependency(err)
}

// ListObjects returns a bounded page and converts provider keys back to logical keys.
func (s *Storage) ListObjects(ctx context.Context, prefix, cursor string, limit int) (appmedia.ObjectPage, error) {
	if limit < 1 {
		return appmedia.ObjectPage{}, apperr.Validation(invalidRequestMessage)
	}
	providerPrefix, err := s.providerPrefix(prefix)
	if err != nil {
		return appmedia.ObjectPage{}, err
	}
	var token *string
	if strings.TrimSpace(cursor) != "" {
		token = aws.String(cursor)
	}
	out, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:            aws.String(s.bucket),
		Prefix:            aws.String(providerPrefix),
		ContinuationToken: token,
		MaxKeys:           aws.Int32(int32(limit)),
	})
	if err != nil {
		return appmedia.ObjectPage{}, mapProviderDependency(err)
	}
	page := appmedia.ObjectPage{}
	for _, item := range out.Contents {
		key := aws.ToString(item.Key)
		if logical, ok := s.logicalKey(key); ok {
			page.Keys = append(page.Keys, logical)
			if item.LastModified != nil {
				page.LastModified = append(page.LastModified, item.LastModified.UTC())
			} else {
				page.LastModified = append(page.LastModified, time.Time{})
			}
		}
	}
	page.NextCursor = aws.ToString(out.NextContinuationToken)
	return page, nil
}

func (s *Storage) providerPrefix(prefix string) (string, error) {
	if strings.TrimSpace(prefix) == "" {
		return "", apperr.Validation(invalidRequestMessage)
	}
	if _, err := s.providerKey(prefix + "x"); err != nil {
		return "", err
	}
	if s.basePath == "" {
		return prefix, nil
	}
	return s.basePath + "/" + prefix, nil
}

func (s *Storage) logicalKey(providerKey string) (string, bool) {
	if s.basePath == "" {
		return providerKey, true
	}
	prefix := s.basePath + "/"
	if !strings.HasPrefix(providerKey, prefix) {
		return "", false
	}
	return strings.TrimPrefix(providerKey, prefix), true
}

func (s *Storage) providerKey(logical string) (string, error) {
	key := strings.TrimSpace(logical)
	if key == "" {
		return "", apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field:   "objectKey",
			Message: "Nesne anahtarı zorunludur.",
		})
	}
	if strings.Contains(key, `\`) || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return "", apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field:   "objectKey",
			Message: "Nesne anahtarı geçersiz.",
		})
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." {
			return "", apperr.Validation(invalidRequestMessage, apperr.FieldError{
				Field:   "objectKey",
				Message: "Nesne anahtarı geçersiz.",
			})
		}
	}
	if s.basePath == "" {
		return key, nil
	}
	return s.basePath + "/" + key, nil
}

var _ appmedia.Storage = (*Storage)(nil)
