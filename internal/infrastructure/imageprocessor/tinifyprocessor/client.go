package tinifyprocessor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	maxErrorBodyBytes  = 4 << 10  // 4 KiB
	maxOutputBodyBytes = 32 << 20 // 32 MiB
)

type shrinkResult struct {
	Bytes       []byte
	ContentType string
	Width       int
	Height      int
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type tinifyClient struct {
	apiKey  string
	baseURL *url.URL
	http    httpDoer
}

func (c *tinifyClient) shrink(ctx context.Context, image []byte) (shrinkResult, error) {
	if err := ctx.Err(); err != nil {
		return shrinkResult{}, err
	}
	if len(image) == 0 {
		return shrinkResult{}, validationImage(invalidImageMessage, "file")
	}

	shrinkURL := c.baseURL.ResolveReference(&url.URL{Path: "/shrink"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, shrinkURL.String(), bytes.NewReader(image))
	if err != nil {
		return shrinkResult{}, dependencyError()
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.SetBasicAuth("api", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return shrinkResult{}, mapTransportError(err)
	}
	defer drainAndClose(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		_ = readErrorBody(resp.Body)
		return shrinkResult{}, mapTinifyStatus(resp.StatusCode)
	}

	loc, err := validateOutputLocation(c.baseURL, resp.Header.Get("Location"))
	if err != nil {
		return shrinkResult{}, err
	}

	return c.downloadOutput(ctx, loc)
}

func (c *tinifyClient) downloadOutput(ctx context.Context, loc *url.URL) (shrinkResult, error) {
	if err := ctx.Err(); err != nil {
		return shrinkResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loc.String(), nil)
	if err != nil {
		return shrinkResult{}, dependencyError()
	}
	req.SetBasicAuth("api", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return shrinkResult{}, mapTransportError(err)
	}
	defer drainAndClose(resp.Body)

	if resp.StatusCode != http.StatusOK {
		_ = readErrorBody(resp.Body)
		return shrinkResult{}, mapTinifyStatus(resp.StatusCode)
	}

	ct, err := canonicalContentType(resp.Header.Get("Content-Type"))
	if err != nil {
		return shrinkResult{}, dependencyError()
	}
	width, err := parsePositiveHeader(resp.Header, "Image-Width")
	if err != nil {
		return shrinkResult{}, dependencyError()
	}
	height, err := parsePositiveHeader(resp.Header, "Image-Height")
	if err != nil {
		return shrinkResult{}, dependencyError()
	}

	body, err := readLimited(resp.Body, maxOutputBodyBytes)
	if err != nil {
		return shrinkResult{}, dependencyError()
	}
	if len(body) == 0 {
		return shrinkResult{}, dependencyError()
	}

	return shrinkResult{
		Bytes:       body,
		ContentType: ct,
		Width:       width,
		Height:      height,
	}, nil
}

func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, maxErrorBodyBytes))
	_ = body.Close()
}

func readErrorBody(r io.Reader) []byte {
	data, _ := readLimited(r, maxErrorBodyBytes)
	return data
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	limited := io.LimitReader(r, limit+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response body exceeds limit")
	}
	return data, nil
}

func parsePositiveHeader(headers http.Header, name string) (int, error) {
	raw := strings.TrimSpace(headers.Get(name))
	if raw == "" {
		return 0, fmt.Errorf("missing header")
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid header")
	}
	return n, nil
}

func sameOrigin(base, loc *url.URL) bool {
	if base == nil || loc == nil {
		return false
	}
	return strings.EqualFold(base.Scheme, loc.Scheme) && strings.EqualFold(base.Host, loc.Host)
}

func validateOutputLocation(base *url.URL, location string) (*url.URL, error) {
	raw := strings.TrimSpace(location)
	if raw == "" {
		return nil, dependencyError()
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, dependencyError()
	}
	if u.Scheme != "https" {
		return nil, dependencyError()
	}
	if u.User != nil || u.Fragment != "" {
		return nil, dependencyError()
	}
	if !sameOrigin(base, u) {
		return nil, dependencyError()
	}
	return u, nil
}

func canonicalContentType(raw string) (string, error) {
	v := strings.TrimSpace(strings.ToLower(raw))
	if v == "" {
		return "", fmt.Errorf("empty content type")
	}
	if i := strings.IndexByte(v, ';'); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	switch v {
	case "image/jpeg", "image/png", "image/webp":
		return v, nil
	default:
		return "", fmt.Errorf("unsupported content type")
	}
}
