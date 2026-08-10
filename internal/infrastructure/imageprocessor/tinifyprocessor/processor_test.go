package tinifyprocessor

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

const testAPIKey = "unit-test-api-key-not-real"

func testProfiles() map[string]ProfileConfig {
	return map[string]ProfileConfig{
		domainmedia.ProfileDetail:   {Width: 100, Height: 100},
		domainmedia.ProfileHomepage: {Width: 80, Height: 60},
		domainmedia.ProfileSearch:   {Width: 40, Height: 40},
	}
}

func testConfig(baseURL string) Config {
	return Config{
		APIKey:      testAPIKey,
		BaseURL:     baseURL,
		HTTPTimeout: 5 * time.Second,
		Profiles:    testProfiles(),
	}
}

func TestNewValidatesConfigWithoutNetwork(t *testing.T) {
	t.Parallel()

	_, err := New(Config{
		APIKey:      "",
		BaseURL:     "https://api.example.invalid",
		HTTPTimeout: time.Second,
		Profiles:    testProfiles(),
	})
	if err == nil {
		t.Fatal("expected missing key error")
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("error leaked key: %v", err)
	}

	_, err = New(Config{
		APIKey:      testAPIKey,
		BaseURL:     "http://api.example.invalid",
		HTTPTimeout: time.Second,
		Profiles:    testProfiles(),
	})
	if err == nil {
		t.Fatal("expected http base URL error")
	}
}

func TestValidateAndNormalizeJPEGAndPNG(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		raw  []byte
		ct   string
	}{
		{name: "jpeg", raw: mustEncodeJPEG(t, 32, 24), ct: "image/jpeg"},
		{name: "png", raw: mustEncodePNG(t, 32, 24), ct: "image/png"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var shrinkCalls atomic.Int32
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, pass, ok := r.BasicAuth()
				if !ok || user != "api" || pass != testAPIKey {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/shrink":
					shrinkCalls.Add(1)
					body, _ := io.ReadAll(r.Body)
					if len(body) == 0 {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if r.Header.Get("Content-Type") != "application/octet-stream" {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					w.Header().Set("Location", "https://"+r.Host+"/out/"+tc.name)
					w.WriteHeader(http.StatusCreated)
				case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/out/"):
					w.Header().Set("Content-Type", tc.ct)
					w.Header().Set("Image-Width", "32")
					w.Header().Set("Image-Height", "24")
					_, _ = w.Write([]byte("compressed-" + tc.name))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			p, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
			if err != nil {
				t.Fatal(err)
			}
			out, err := p.ValidateAndNormalize(context.Background(), tc.raw, tc.ct+"; charset=binary")
			if err != nil {
				t.Fatal(err)
			}
			if out.ContentType != tc.ct || out.Width != 32 || out.Height != 24 {
				t.Fatalf("out=%+v", out)
			}
			if string(out.Bytes) != "compressed-"+tc.name {
				t.Fatalf("bytes=%q", out.Bytes)
			}
			if shrinkCalls.Load() != 1 {
				t.Fatalf("shrink calls=%d", shrinkCalls.Load())
			}
		})
	}
}

func TestValidateAndNormalizeLocalFailures(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("network should not be called")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	p, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := p.ValidateAndNormalize(context.Background(), nil, "image/jpeg"); err == nil {
		t.Fatal("expected empty raw error")
	} else if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindValidation {
		t.Fatalf("want validation, got %v", err)
	}

	if _, err := p.ValidateAndNormalize(context.Background(), []byte("not-an-image"), "image/jpeg"); err == nil {
		t.Fatal("expected corrupt error")
	}

	jpeg := mustEncodeJPEG(t, 16, 16)
	if _, err := p.ValidateAndNormalize(context.Background(), jpeg, "image/png"); err == nil {
		t.Fatal("expected declared mismatch")
	}

	gifLike := []byte("GIF89a")
	if _, err := p.ValidateAndNormalize(context.Background(), gifLike, ""); err == nil {
		t.Fatal("expected unsupported mime")
	}
}

func TestValidateAndNormalizeRejectsForeignLocation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/shrink" {
			w.Header().Set("Location", "https://evil.example/steal")
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.ValidateAndNormalize(context.Background(), mustEncodeJPEG(t, 8, 8), "image/jpeg")
	if err == nil {
		t.Fatal("expected foreign location rejection")
	}
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("want dependency, got %v", err)
	}
	if strings.Contains(err.Error(), "evil.example") || strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("error leaked secret/url: %v", err)
	}
}

func TestValidateAndNormalizeStatusMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		kind   apperr.Kind
	}{
		{name: "400", status: 400, kind: apperr.KindValidation},
		{name: "401", status: 401, kind: apperr.KindDependencyUnavailable},
		{name: "403", status: 403, kind: apperr.KindDependencyUnavailable},
		{name: "429", status: 429, kind: apperr.KindDependencyUnavailable},
		{name: "500", status: 500, kind: apperr.KindDependencyUnavailable},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/shrink" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(`{"error":"BadRequest","message":"provider-secret-body"}`))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer srv.Close()

			p, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = p.ValidateAndNormalize(context.Background(), mustEncodeJPEG(t, 8, 8), "")
			if err == nil {
				t.Fatal("expected mapped error")
			}
			ae, ok := apperr.As(err)
			if !ok || ae.Kind != tc.kind {
				t.Fatalf("kind=%v err=%v", ae, err)
			}
			if strings.Contains(err.Error(), "provider-secret-body") || strings.Contains(err.Error(), testAPIKey) {
				t.Fatalf("leaked provider body/key: %v", err)
			}
		})
	}
}

func TestValidateAndNormalizeMissingLocationAndBadOutput(t *testing.T) {
	t.Parallel()

	t.Run("missing_location", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		defer srv.Close()
		p, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.ValidateAndNormalize(context.Background(), mustEncodeJPEG(t, 8, 8), ""); err == nil {
			t.Fatal("expected missing location error")
		}
	})

	t.Run("non_201", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		p, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.ValidateAndNormalize(context.Background(), mustEncodeJPEG(t, 8, 8), ""); err == nil {
			t.Fatal("expected non-201 error")
		}
	})

	t.Run("empty_output_body", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				w.Header().Set("Location", "https://"+r.Host+"/out")
				w.WriteHeader(http.StatusCreated)
				return
			}
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Image-Width", "8")
			w.Header().Set("Image-Height", "8")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		p, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.ValidateAndNormalize(context.Background(), mustEncodeJPEG(t, 8, 8), ""); err == nil {
			t.Fatal("expected empty body error")
		}
	})

	t.Run("output_mime_mismatch", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				w.Header().Set("Location", "https://"+r.Host+"/out")
				w.WriteHeader(http.StatusCreated)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Image-Width", "8")
			w.Header().Set("Image-Height", "8")
			_, _ = w.Write([]byte("x"))
		}))
		defer srv.Close()
		p, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.ValidateAndNormalize(context.Background(), mustEncodeJPEG(t, 8, 8), ""); err == nil {
			t.Fatal("expected mime mismatch error")
		}
	})
}

func TestValidateAndNormalizeContextCanceled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	p, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = p.ValidateAndNormalize(ctx, mustEncodeJPEG(t, 8, 8), "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want canceled, got %v", err)
	}
}

func TestGenerateVariantProfilesAndSingleShrink(t *testing.T) {
	t.Parallel()

	master := mustEncodeJPEG(t, 200, 100)
	var shrinkCalls atomic.Int32
	var lastBodyLen atomic.Int32

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "api" || pass != testAPIKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/shrink":
			shrinkCalls.Add(1)
			body, _ := io.ReadAll(r.Body)
			lastBodyLen.Store(int32(len(body)))
			// Tinify server-side resize endpoint must not be used.
			if strings.Contains(r.URL.RawQuery, "resize") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Location", "https://"+r.Host+"/out")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/out":
			// DETAIL fit of 200x100 into 100x100 => 100x50
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Image-Width", "100")
			w.Header().Set("Image-Height", "50")
			_, _ = w.Write([]byte("variant-bytes"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := p.GenerateVariant(context.Background(), master, "THUMB"); err == nil {
		t.Fatal("expected unknown profile")
	}

	out, err := p.GenerateVariant(context.Background(), master, domainmedia.ProfileDetail)
	if err != nil {
		t.Fatal(err)
	}
	if out.Width != 100 || out.Height != 50 || out.ContentType != "image/jpeg" {
		t.Fatalf("out=%+v", out)
	}
	if string(out.Bytes) != "variant-bytes" {
		t.Fatalf("bytes=%q", out.Bytes)
	}
	if shrinkCalls.Load() != 1 {
		t.Fatalf("expected one shrink, got %d", shrinkCalls.Load())
	}
	if lastBodyLen.Load() == int32(len(master)) {
		t.Fatal("expected locally resized body, got original master size")
	}
	if lastBodyLen.Load() == 0 {
		t.Fatal("empty shrink body")
	}
}

func TestGenerateVariantHomepageAndSearchDims(t *testing.T) {
	t.Parallel()

	master := mustEncodePNG(t, 160, 120)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Location", "https://"+r.Host+"/out")
			w.WriteHeader(http.StatusCreated)
			return
		}
		// HOMEPAGE 80x60 fit of 160x120 => 80x60
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Image-Width", "80")
		w.Header().Set("Image-Height", "60")
		_, _ = w.Write([]byte("hp"))
	}))
	defer srv.Close()
	p, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.GenerateVariant(context.Background(), master, domainmedia.ProfileHomepage)
	if err != nil {
		t.Fatal(err)
	}
	if out.Width != 80 || out.Height != 60 {
		t.Fatalf("homepage out=%+v", out)
	}

	srv2 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Location", "https://"+r.Host+"/out")
			w.WriteHeader(http.StatusCreated)
			return
		}
		// SEARCH 40x40 fit of 160x120 => 40x30
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Image-Width", "40")
		w.Header().Set("Image-Height", "30")
		_, _ = w.Write([]byte("search"))
	}))
	defer srv2.Close()
	p2, err := newWithHTTPClient(testConfig(srv2.URL), srv2.Client())
	if err != nil {
		t.Fatal(err)
	}
	out2, err := p2.GenerateVariant(context.Background(), master, domainmedia.ProfileSearch)
	if err != nil {
		t.Fatal(err)
	}
	if out2.Width != 40 || out2.Height != 30 {
		t.Fatalf("search out=%+v", out2)
	}
}

func TestCompileTimeImageProcessor(t *testing.T) {
	t.Parallel()
	var _ appmedia.ImageProcessor = (*Processor)(nil)
}
