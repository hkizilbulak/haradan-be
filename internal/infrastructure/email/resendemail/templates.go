package resendemail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// TemplateSummary is a Resend template list entry (id/alias/name/status).
type TemplateSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Alias  string `json:"alias,omitempty"`
	Status string `json:"status,omitempty"`
}

// ListTemplates calls GET /templates?limit=100.
func (c *resendClient) ListTemplates(ctx context.Context) ([]TemplateSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	endpoint := c.baseURL.ResolveReference(&url.URL{Path: templatesPath, RawQuery: "limit=100"})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, dependencyError()
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, mapTransportError(err)
	}
	defer drainAndClose(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_ = readErrorBody(resp.Body)
		return nil, mapResendStatus(resp.StatusCode)
	}
	raw, err := readLimited(resp.Body, 1<<20)
	if err != nil {
		return nil, dependencyError()
	}
	var parsed struct {
		Data []TemplateSummary `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, dependencyError()
	}
	return parsed.Data, nil
}

// GetTemplateVariables calls GET /templates/{id} and extracts {{var}} names.
// When the id is an alias/name miss, it resolves via ListTemplates once.
func (c *resendClient) GetTemplateVariables(ctx context.Context, idOrAlias string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(idOrAlias)
	if id == "" || containsCRLF(id) {
		return nil, dependencyError()
	}
	body, status, err := c.fetchTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		resolved, resolveErr := c.resolveTemplateID(ctx, id)
		if resolveErr != nil {
			return nil, resolveErr
		}
		body, status, err = c.fetchTemplate(ctx, resolved)
		if err != nil {
			return nil, err
		}
	}
	if status < 200 || status > 299 {
		return nil, mapResendStatus(status)
	}
	return extractTemplateVariables(body), nil
}

func (c *resendClient) fetchTemplate(ctx context.Context, id string) ([]byte, int, error) {
	endpoint := c.baseURL.ResolveReference(&url.URL{Path: templatesPath + "/" + url.PathEscape(id)})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, 0, dependencyError()
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, mapTransportError(err)
	}
	defer drainAndClose(resp.Body)
	raw, err := readLimited(resp.Body, 1<<20)
	if err != nil {
		return nil, resp.StatusCode, dependencyError()
	}
	return raw, resp.StatusCode, nil
}

func (c *resendClient) resolveTemplateID(ctx context.Context, idOrAlias string) (string, error) {
	list, err := c.ListTemplates(ctx)
	if err != nil {
		return "", err
	}
	for _, t := range list {
		if t.ID == idOrAlias || t.Alias == idOrAlias || t.Name == idOrAlias {
			return t.ID, nil
		}
	}
	return "", mapResendStatus(http.StatusNotFound)
}

var templateVarPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

func extractTemplateVariables(raw []byte) []string {
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil
	}
	blobs := make([]string, 0, 4)
	for _, key := range []string{"html", "text", "subject"} {
		if v, ok := envelope[key].(string); ok {
			blobs = append(blobs, v)
		}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, blob := range blobs {
		for _, m := range templateVarPattern.FindAllStringSubmatch(blob, -1) {
			if len(m) < 2 {
				continue
			}
			name := m[1]
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}

// ListTemplates is the exported discovery method on Sender.
func (s *Sender) ListTemplates(ctx context.Context) ([]TemplateSummary, error) {
	if s == nil || s.client == nil {
		return nil, dependencyError()
	}
	out, err := s.client.ListTemplates(ctx)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	return out, nil
}

// GetTemplateVariables is the exported discovery method on Sender.
func (s *Sender) GetTemplateVariables(ctx context.Context, idOrAlias string) ([]string, error) {
	if s == nil || s.client == nil {
		return nil, dependencyError()
	}
	out, err := s.client.GetTemplateVariables(ctx, idOrAlias)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	return out, nil
}
