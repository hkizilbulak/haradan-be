package tinifyprocessor

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"
	"time"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

// Processor implements appmedia.ImageProcessor using Tinify shrink plus local fit resize.
type Processor struct {
	client   *tinifyClient
	profiles map[string]ProfileConfig
}

var _ appmedia.ImageProcessor = (*Processor)(nil)

// New builds a Processor with a shared HTTP client. It performs no network I/O.
func New(cfg Config) (*Processor, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	baseURL, err := parseBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Timeout: cfg.HTTPTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			// Never auto-follow: Location targets are validated explicitly.
			return http.ErrUseLastResponse
		},
	}

	profiles := make(map[string]ProfileConfig, len(cfg.Profiles))
	for k, v := range cfg.Profiles {
		profiles[k] = v
	}

	return &Processor{
		client: &tinifyClient{
			apiKey:  strings.TrimSpace(cfg.APIKey),
			baseURL: baseURL,
			http:    httpClient,
		},
		profiles: profiles,
	}, nil
}

// newWithHTTPClient is used by unit tests to inject a fake transport.
func newWithHTTPClient(cfg Config, doer httpDoer) (*Processor, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	baseURL, err := parseBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	if doer == nil {
		return nil, fmt.Errorf("http client is required")
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 30 * time.Second
	}
	profiles := make(map[string]ProfileConfig, len(cfg.Profiles))
	for k, v := range cfg.Profiles {
		profiles[k] = v
	}
	return &Processor{
		client: &tinifyClient{
			apiKey:  strings.TrimSpace(cfg.APIKey),
			baseURL: baseURL,
			http:    doer,
		},
		profiles: profiles,
	}, nil
}

// ValidateAndNormalize validates raw bytes locally and compresses via Tinify shrink.
func (p *Processor) ValidateAndNormalize(
	ctx context.Context,
	raw []byte,
	declaredType string,
) (appmedia.ProcessedImage, error) {
	if err := ctx.Err(); err != nil {
		return appmedia.ProcessedImage{}, err
	}
	src, err := validateRawImage(raw, declaredType)
	if err != nil {
		return appmedia.ProcessedImage{}, err
	}

	out, err := p.client.shrink(ctx, raw)
	if err != nil {
		return appmedia.ProcessedImage{}, sanitizeErr(err)
	}
	if out.ContentType != src.ContentType {
		return appmedia.ProcessedImage{}, dependencyError()
	}
	if out.Width <= 0 || out.Height <= 0 || len(out.Bytes) == 0 {
		return appmedia.ProcessedImage{}, dependencyError()
	}
	return appmedia.ProcessedImage{
		ContentType: out.ContentType,
		Bytes:       out.Bytes,
		Width:       out.Width,
		Height:      out.Height,
	}, nil
}

// GenerateVariant locally fit-resizes the master, then compresses once via Tinify.
func (p *Processor) GenerateVariant(
	ctx context.Context,
	master []byte,
	profile string,
) (appmedia.ProcessedImage, error) {
	if err := ctx.Err(); err != nil {
		return appmedia.ProcessedImage{}, err
	}
	if !domainmedia.IsKnownTransformProfile(profile) {
		return appmedia.ProcessedImage{}, apperr.Validation(invalidProfileMessage, apperr.FieldError{
			Field:   "profile",
			Message: invalidProfileMessage,
		})
	}
	bounds, ok := p.profiles[profile]
	if !ok || bounds.Width <= 0 || bounds.Height <= 0 {
		return appmedia.ProcessedImage{}, apperr.DependencyUnavailable(processorMisconfiguredMessage)
	}

	src, err := decodeImage(master)
	if err != nil {
		return appmedia.ProcessedImage{}, err
	}

	resized, w, h, err := resizeFit(src, bounds.Width, bounds.Height)
	if err != nil {
		return appmedia.ProcessedImage{}, validationImage(invalidImageMessage, "file")
	}

	out, err := p.client.shrink(ctx, resized)
	if err != nil {
		return appmedia.ProcessedImage{}, sanitizeErr(err)
	}
	if out.ContentType != src.ContentType {
		return appmedia.ProcessedImage{}, dependencyError()
	}
	if out.Width != w || out.Height != h {
		return appmedia.ProcessedImage{}, dependencyError()
	}
	if len(out.Bytes) == 0 {
		return appmedia.ProcessedImage{}, dependencyError()
	}
	return appmedia.ProcessedImage{
		ContentType: out.ContentType,
		Bytes:       out.Bytes,
		Width:       out.Width,
		Height:      out.Height,
	}, nil
}

func validateRawImage(raw []byte, declaredType string) (decodedImage, error) {
	if len(raw) == 0 {
		return decodedImage{}, validationImage(invalidImageMessage, "file")
	}

	sniffed := http.DetectContentType(raw)
	actual, err := canonicalContentType(sniffed)
	if err != nil {
		return decodedImage{}, validationImage(unsupportedImageMessage, "contentType")
	}

	if strings.TrimSpace(declaredType) != "" {
		declared, derr := canonicalContentType(declaredType)
		if derr != nil || declared != actual {
			return decodedImage{}, validationImage(unsupportedImageMessage, "contentType")
		}
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return decodedImage{}, validationImage(invalidImageMessage, "file")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return decodedImage{}, validationImage(invalidImageMessage, "file")
	}

	return decodedImage{
		ContentType: actual,
		Width:       cfg.Width,
		Height:      cfg.Height,
	}, nil
}

func decodeImage(raw []byte) (decodedImage, error) {
	meta, err := validateRawImage(raw, "")
	if err != nil {
		return decodedImage{}, err
	}
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return decodedImage{}, validationImage(invalidImageMessage, "file")
	}
	var expected string
	switch format {
	case "jpeg":
		expected = "image/jpeg"
	case "png":
		expected = "image/png"
	default:
		return decodedImage{}, validationImage(unsupportedImageMessage, "contentType")
	}
	if expected != meta.ContentType {
		return decodedImage{}, validationImage(invalidImageMessage, "file")
	}
	meta.Img = img
	return meta, nil
}
