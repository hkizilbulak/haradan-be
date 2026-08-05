package s3storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

type fakeObjectAPI struct {
	headIn  *s3.HeadObjectInput
	headOut *s3.HeadObjectOutput
	headErr error

	putIn  *s3.PutObjectInput
	putErr error

	getIn  *s3.GetObjectInput
	getOut *s3.GetObjectOutput
	getErr error

	deleteIn  *s3.DeleteObjectInput
	deleteErr error

	listIn  *s3.ListObjectsV2Input
	listOut *s3.ListObjectsV2Output
	listErr error
}

func (f *fakeObjectAPI) HeadObject(_ context.Context, params *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	f.headIn = params
	return f.headOut, f.headErr
}

func (f *fakeObjectAPI) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.putIn = params
	if params != nil && params.Body != nil {
		_, _ = io.ReadAll(params.Body)
	}
	return &s3.PutObjectOutput{}, f.putErr
}

func (f *fakeObjectAPI) GetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.getIn = params
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getOut, nil
}

func (f *fakeObjectAPI) DeleteObject(_ context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.deleteIn = params
	return &s3.DeleteObjectOutput{}, f.deleteErr
}

func (f *fakeObjectAPI) ListObjectsV2(_ context.Context, params *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	f.listIn = params
	return f.listOut, f.listErr
}

func TestDeleteObjectUsesLogicalKey(t *testing.T) {
	api := &fakeObjectAPI{}
	store := mustStore(t, validCfg(), api, &fakePresigner{}, nil)
	if err := store.DeleteObject(context.Background(), "assets/a/raw"); err != nil {
		t.Fatal(err)
	}
	if got := aws.ToString(api.deleteIn.Key); got != "assets/a/raw" {
		t.Fatalf("delete key=%q", got)
	}
}

func TestListObjectsConvertsProviderPrefix(t *testing.T) {
	api := &fakeObjectAPI{listOut: &s3.ListObjectsV2Output{
		Contents:              []types.Object{{Key: aws.String("assets/a/raw")}},
		NextContinuationToken: aws.String("next"),
	}}
	store := mustStore(t, validCfg(), api, &fakePresigner{}, nil)
	page, err := store.ListObjects(context.Background(), "assets/", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := aws.ToString(api.listIn.Prefix); got != "assets/" {
		t.Fatalf("list prefix=%q", got)
	}
	if len(page.Keys) != 1 || page.Keys[0] != "assets/a/raw" || page.NextCursor != "next" {
		t.Fatalf("page=%+v", page)
	}
}

// fakePresigner records PresignPutObject inputs for unit tests only.
type fakePresigner struct {
	lastBucket             string
	lastKey                string
	lastContentType        *string
	lastExpires            time.Duration
	lastContentLengthRange string
	resultURL              string
	resultMethod           string
	err                    error
}

func (f *fakePresigner) PresignPutObject(
	_ context.Context,
	params *s3.PutObjectInput,
	optFns ...func(*s3.PresignOptions),
) (*v4PresignedHTTPRequest, error) {
	if params != nil {
		f.lastBucket = aws.ToString(params.Bucket)
		f.lastKey = aws.ToString(params.Key)
		f.lastContentType = params.ContentType
	}
	opts := &s3.PresignOptions{}
	for _, fn := range optFns {
		if fn != nil {
			fn(opts)
		}
	}
	f.lastExpires = opts.Expires
	f.lastContentLengthRange = ""
	if f.err != nil {
		return nil, f.err
	}
	method := f.resultMethod
	if method == "" {
		method = "PUT"
	}
	return &v4PresignedHTTPRequest{URL: f.resultURL, Method: method}, nil
}

type trackingBody struct {
	*bytes.Reader
	closed atomic.Bool
	err    error
}

func (t *trackingBody) Read(p []byte) (int, error) {
	if t.err != nil {
		return 0, t.err
	}
	return t.Reader.Read(p)
}

func (t *trackingBody) Close() error {
	t.closed.Store(true)
	return nil
}

type apiError struct {
	code    string
	message string
}

func (e apiError) Error() string                 { return e.message }
func (e apiError) ErrorCode() string             { return e.code }
func (e apiError) ErrorMessage() string          { return e.message }
func (e apiError) ErrorFault() smithy.ErrorFault { return smithy.FaultUnknown }

func validCfg() Config {
	return Config{
		Endpoint:  "https://example.invalid",
		Region:    "eu-central-003",
		Bucket:    "media-bucket",
		AccessKey: "AKIA_TEST_KEY",
		SecretKey: "SECRET_TEST_VALUE_DO_NOT_LEAK",
		BasePath:  "",
	}
}

func mustStore(t *testing.T, cfg Config, api objectAPI, presign presignAPI, now func() time.Time) *Storage {
	t.Helper()
	store, err := newWithDeps(cfg, api, presign, now)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestConfigValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		mut  func(*Config)
	}{
		{"endpoint", func(c *Config) { c.Endpoint = "" }},
		{"region", func(c *Config) { c.Region = "" }},
		{"bucket", func(c *Config) { c.Bucket = "" }},
		{"access", func(c *Config) { c.AccessKey = "" }},
		{"secret", func(c *Config) { c.SecretKey = "" }},
		{"base traversal", func(c *Config) { c.BasePath = "a/../b" }},
		{"base slash", func(c *Config) { c.BasePath = "/media" }},
		{"base backslash", func(c *Config) { c.BasePath = `media\x` }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validCfg()
			tc.mut(&cfg)
			_, err := newWithDeps(cfg, &fakeObjectAPI{}, &fakePresigner{}, time.Now)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if strings.Contains(err.Error(), "SECRET_TEST_VALUE") {
				t.Fatalf("error leaked secret: %v", err)
			}
		})
	}
}

func TestCreateUploadAuthorization(t *testing.T) {
	t.Parallel()
	fixed := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	presign := &fakePresigner{
		resultURL:    "https://signed.example/put",
		resultMethod: "PUT",
	}
	store := mustStore(t, validCfg(), &fakeObjectAPI{}, presign, func() time.Time { return fixed })

	auth, err := store.CreateUploadAuthorization(
		context.Background(),
		"assets/11111111-1111-1111-1111-111111111111/raw",
		"image/png",
		1024,
		5*time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Method != "PUT" {
		t.Fatalf("method=%q", auth.Method)
	}
	if auth.URL != "https://signed.example/put" {
		t.Fatalf("url=%q", auth.URL)
	}
	if auth.Headers["Content-Type"] != "image/png" {
		t.Fatalf("headers=%v", auth.Headers)
	}
	if auth.ObjectKey != "assets/11111111-1111-1111-1111-111111111111/raw" {
		t.Fatalf("objectKey=%q", auth.ObjectKey)
	}
	if !auth.ExpiresAt.Equal(fixed.Add(5 * time.Minute)) {
		t.Fatalf("expiresAt=%v", auth.ExpiresAt)
	}
	if presign.lastKey != "assets/11111111-1111-1111-1111-111111111111/raw" {
		t.Fatalf("provider key=%q", presign.lastKey)
	}
	if aws.ToString(presign.lastContentType) != "image/png" {
		t.Fatalf("contentType=%v", presign.lastContentType)
	}
	if presign.lastExpires != 5*time.Minute {
		t.Fatalf("expires=%v", presign.lastExpires)
	}
	if presign.lastContentLengthRange != "" {
		t.Fatalf("must not set content-length-range, got %q", presign.lastContentLengthRange)
	}
}

func TestCreateUploadAuthorizationBasePath(t *testing.T) {
	t.Parallel()
	cfg := validCfg()
	cfg.BasePath = "media/prod"
	presign := &fakePresigner{resultURL: "https://signed.example/put", resultMethod: "PUT"}
	store := mustStore(t, cfg, &fakeObjectAPI{}, presign, time.Now)
	_, err := store.CreateUploadAuthorization(context.Background(), "assets/a/raw", "image/jpeg", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if presign.lastKey != "media/prod/assets/a/raw" {
		t.Fatalf("key=%q", presign.lastKey)
	}
}

func TestCreateUploadAuthorizationValidation(t *testing.T) {
	t.Parallel()
	store := mustStore(t, validCfg(), &fakeObjectAPI{}, &fakePresigner{}, time.Now)
	cases := []struct {
		name string
		ct   string
		max  int64
		ttl  time.Duration
	}{
		{"ttl", "image/png", 10, 0},
		{"contentType", "", 10, time.Minute},
		{"maxBytes", "image/png", 0, time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := store.CreateUploadAuthorization(context.Background(), "assets/a/raw", tc.ct, tc.max, tc.ttl)
			ae, ok := apperr.As(err)
			if !ok || ae.Kind != apperr.KindValidation {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestCreateUploadAuthorizationPresignErrorMapped(t *testing.T) {
	t.Parallel()
	presign := &fakePresigner{err: errors.New("signed-url-secret-leak https://evil")}
	store := mustStore(t, validCfg(), &fakeObjectAPI{}, presign, time.Now)
	_, err := store.CreateUploadAuthorization(context.Background(), "assets/a/raw", "image/png", 10, time.Minute)
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "https://") || strings.Contains(err.Error(), "signed-url") {
		t.Fatalf("leaked provider detail: %v", err)
	}
}

func TestProviderKeyRejectsTraversal(t *testing.T) {
	t.Parallel()
	store := mustStore(t, validCfg(), &fakeObjectAPI{}, &fakePresigner{}, time.Now)
	bad := []string{"", "/assets/a/raw", `assets\a\raw`, "assets/../raw", "assets//raw", "assets/./raw"}
	for _, key := range bad {
		_, err := store.providerKey(key)
		ae, ok := apperr.As(err)
		if !ok || ae.Kind != apperr.KindValidation {
			t.Fatalf("key %q: err=%v", key, err)
		}
	}
	okKeys := []string{
		"assets/11111111-1111-1111-1111-111111111111/raw",
		"assets/11111111-1111-1111-1111-111111111111/master",
		"assets/11111111-1111-1111-1111-111111111111/variants/DETAIL",
	}
	for _, key := range okKeys {
		got, err := store.providerKey(key)
		if err != nil || got != key {
			t.Fatalf("key %q: got=%q err=%v", key, got, err)
		}
	}
}

func TestHeadObjectSuccess(t *testing.T) {
	t.Parallel()
	api := &fakeObjectAPI{
		headOut: &s3.HeadObjectOutput{
			ContentType:   aws.String("image/png"),
			ContentLength: aws.Int64(42),
		},
	}
	store := mustStore(t, validCfg(), api, &fakePresigner{}, time.Now)
	info, err := store.HeadObject(context.Background(), "assets/a/raw")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Exists || info.ContentType != "image/png" || info.ByteSize != 42 {
		t.Fatalf("info=%+v", info)
	}
	if aws.ToString(api.headIn.Key) != "assets/a/raw" {
		t.Fatalf("key=%v", api.headIn.Key)
	}
}

func TestHeadObjectNotFoundVariants(t *testing.T) {
	t.Parallel()
	cases := []error{
		&types.NotFound{},
		&types.NoSuchKey{},
		apiError{code: "NotFound", message: "missing"},
		apiError{code: "NoSuchKey", message: "missing"},
		&smithyhttp.ResponseError{Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 404}}},
	}
	for i, cause := range cases {
		api := &fakeObjectAPI{headErr: cause}
		store := mustStore(t, validCfg(), api, &fakePresigner{}, time.Now)
		info, err := store.HeadObject(context.Background(), "assets/a/raw")
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if info.Exists {
			t.Fatalf("case %d: expected Exists=false", i)
		}
	}
}

func TestHeadObjectNoSuchBucketIsDependency(t *testing.T) {
	t.Parallel()
	api := &fakeObjectAPI{headErr: apiError{code: "NoSuchBucket", message: "bucket missing"}}
	store := mustStore(t, validCfg(), api, &fakePresigner{}, time.Now)
	info, err := store.HeadObject(context.Background(), "assets/a/raw")
	if info.Exists {
		t.Fatal("Exists must stay false on error path")
	}
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "NoSuchBucket") || strings.Contains(err.Error(), "bucket missing") {
		t.Fatalf("leaked provider detail: %v", err)
	}
}

func TestHeadObjectAuthAndServerErrors(t *testing.T) {
	t.Parallel()
	cases := []error{
		apiError{code: "AccessDenied", message: "denied SECRET"},
		apiError{code: "InternalError", message: "boom"},
		&smithyhttp.ResponseError{Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 500}}},
	}
	for i, cause := range cases {
		api := &fakeObjectAPI{headErr: cause}
		store := mustStore(t, validCfg(), api, &fakePresigner{}, time.Now)
		_, err := store.HeadObject(context.Background(), "assets/a/raw")
		ae, ok := apperr.As(err)
		if !ok || ae.Kind != apperr.KindDependencyUnavailable {
			t.Fatalf("case %d: err=%v", i, err)
		}
		if strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "AccessDenied") {
			t.Fatalf("case %d leaked: %v", i, err)
		}
	}
}

func TestHeadObjectContextDeadline(t *testing.T) {
	t.Parallel()
	api := &fakeObjectAPI{headErr: context.DeadlineExceeded}
	store := mustStore(t, validCfg(), api, &fakePresigner{}, time.Now)
	_, err := store.HeadObject(context.Background(), "assets/a/raw")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
}

func TestPutObject(t *testing.T) {
	t.Parallel()
	cfg := validCfg()
	cfg.BasePath = "prefix"
	api := &fakeObjectAPI{}
	store := mustStore(t, cfg, api, &fakePresigner{}, time.Now)
	body := []byte("hello-image")
	if err := store.PutObject(context.Background(), "assets/a/master", "image/png", body); err != nil {
		t.Fatal(err)
	}
	if aws.ToString(api.putIn.Bucket) != "media-bucket" {
		t.Fatalf("bucket=%v", api.putIn.Bucket)
	}
	if aws.ToString(api.putIn.Key) != "prefix/assets/a/master" {
		t.Fatalf("key=%v", api.putIn.Key)
	}
	if aws.ToString(api.putIn.ContentType) != "image/png" {
		t.Fatalf("ct=%v", api.putIn.ContentType)
	}
	if api.putIn.ACL != "" {
		t.Fatalf("ACL must stay unset, got %q", api.putIn.ACL)
	}
	if aws.ToInt64(api.putIn.ContentLength) != int64(len(body)) {
		t.Fatalf("contentLength=%v", api.putIn.ContentLength)
	}
}

func TestPutObjectProviderError(t *testing.T) {
	t.Parallel()
	api := &fakeObjectAPI{putErr: errors.New("provider boom")}
	store := mustStore(t, validCfg(), api, &fakePresigner{}, time.Now)
	err := store.PutObject(context.Background(), "assets/a/master", "image/png", []byte("x"))
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("err=%v", err)
	}
}

func TestGetObjectSuccess(t *testing.T) {
	t.Parallel()
	body := &trackingBody{Reader: bytes.NewReader([]byte("payload"))}
	api := &fakeObjectAPI{
		getOut: &s3.GetObjectOutput{
			Body:        body,
			ContentType: aws.String("image/jpeg"),
		},
	}
	store := mustStore(t, validCfg(), api, &fakePresigner{}, time.Now)
	data, ct, err := store.GetObject(context.Background(), "assets/a/master")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "payload" || ct != "image/jpeg" {
		t.Fatalf("data=%q ct=%q", data, ct)
	}
	if !body.closed.Load() {
		t.Fatal("body was not closed")
	}
}

func TestGetObjectNotFound(t *testing.T) {
	t.Parallel()
	api := &fakeObjectAPI{getErr: &types.NoSuchKey{}}
	store := mustStore(t, validCfg(), api, &fakePresigner{}, time.Now)
	_, _, err := store.GetObject(context.Background(), "assets/a/master")
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindNotFound {
		t.Fatalf("err=%v", err)
	}
	if ae.Message != "Nesne bulunamadı." {
		t.Fatalf("message=%q", ae.Message)
	}
}

func TestGetObjectReadFailure(t *testing.T) {
	t.Parallel()
	body := &trackingBody{Reader: bytes.NewReader(nil), err: errors.New("read boom")}
	api := &fakeObjectAPI{
		getOut: &s3.GetObjectOutput{Body: body, ContentType: aws.String("image/png")},
	}
	store := mustStore(t, validCfg(), api, &fakePresigner{}, time.Now)
	_, _, err := store.GetObject(context.Background(), "assets/a/master")
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindInternal {
		t.Fatalf("err=%v", err)
	}
	if !body.closed.Load() {
		t.Fatal("body must close on read failure")
	}
	if strings.Contains(err.Error(), "read boom") {
		// Internal client message is fixed; cause is wrapped but Error() uses safe Message.
	}
	if ae.Message != "Beklenmeyen bir hata oluştu." {
		t.Fatalf("message=%q", ae.Message)
	}
}

func TestGetObjectContextCanceled(t *testing.T) {
	t.Parallel()
	api := &fakeObjectAPI{getErr: context.Canceled}
	store := mustStore(t, validCfg(), api, &fakePresigner{}, time.Now)
	_, _, err := store.GetObject(context.Background(), "assets/a/master")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestInterfaceSatisfaction(t *testing.T) {
	t.Parallel()
	var _ appmedia.Storage = (*Storage)(nil)
}
