package resendemail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	maxErrorBodyBytes = 4 << 10 // 4 KiB
	emailsPath        = "/emails"
	templatesPath     = "/templates"
)

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type resendClient struct {
	apiKey  string
	baseURL *url.URL
	http    httpDoer
	from    string
	logger  resendLogger
}

type resendLogger interface {
	Error(msg string, args ...any)
}

type sendTemplateRequest struct {
	From     string              `json:"from"`
	To       []string            `json:"to"`
	Template sendTemplatePayload `json:"template"`
}

type sendTemplatePayload struct {
	ID        string         `json:"id"`
	Variables map[string]any `json:"variables"`
}

func (c *resendClient) sendTemplate(
	ctx context.Context,
	toAddress string,
	templateID string,
	variables map[string]any,
) error {
	return c.sendTemplateWithIdempotency(ctx, toAddress, templateID, variables, "")
}

func (c *resendClient) sendTemplateWithIdempotency(
	ctx context.Context,
	toAddress string,
	templateID string,
	variables map[string]any,
	idempotencyKey string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	payload := sendTemplateRequest{
		From: c.from,
		To:   []string{toAddress},
		Template: sendTemplatePayload{
			ID:        templateID,
			Variables: variables,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return dependencyError()
	}

	endpoint := c.baseURL.ResolveReference(&url.URL{Path: emailsPath})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return dependencyError()
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return mapTransportError(err)
	}
	defer drainAndClose(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return nil
	}
	if resp.StatusCode == 409 {
		// Duplicate idempotency key: treat as success per provider contract.
		return nil
	}
	errBody := readErrorBody(resp.Body)
	if c.logger != nil && len(errBody) > 0 {
		c.logger.Error("resend API error", "status", resp.StatusCode, "body", string(errBody))
	}
	return mapResendStatus(resp.StatusCode)
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
