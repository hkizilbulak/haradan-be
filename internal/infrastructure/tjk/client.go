// Package tjk implements the bounded, redirect-free HTTP adapter for the
// public TJK horse pages. The HTML layout is not an API contract, so parsing is
// deliberately isolated behind Parser and kept tolerant of extra columns.
package tjk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const defaultMaxBodyBytes int64 = 2 << 20

var ErrUnconfigured = errors.New("tjk adapter is not configured")

type Horse struct {
	Number string
	Name   string
	Race   string
	Sire   string
	Dam    string
}

// Parser permits fixture-driven tests and future TJK markup adaptations.
type Parser interface {
	Parse(io.Reader) ([]Horse, error)
}

type Config struct {
	BaseURL      string
	HTTPTimeout  time.Duration
	MaxBodyBytes int64
	HTTPClient   *http.Client
	Parser       Parser
}

type Client struct {
	baseURL *url.URL
	http    *http.Client
	maxBody int64
	parser  Parser
}

func NewClient(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, ErrUnconfigured
	}
	u, err := url.Parse(strings.TrimSpace(cfg.BaseURL))
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
		return nil, fmt.Errorf("invalid TJK base URL")
	}
	if cfg.HTTPTimeout <= 0 {
		return nil, fmt.Errorf("TJK HTTP timeout must be greater than zero")
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMaxBodyBytes
	}
	if cfg.Parser == nil {
		cfg.Parser = HTMLTableParser{}
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}
	copyClient := *hc
	copyClient.Timeout = cfg.HTTPTimeout
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Client{baseURL: u, http: &copyClient, maxBody: cfg.MaxBodyBytes, parser: cfg.Parser}, nil
}

// FetchPage requests the configured query page. Cursor is emitted as the
// page query parameter; deployments may point BaseURL at a test-compatible
// endpoint without changing code.
func (c *Client) FetchPage(ctx context.Context, cursor string) ([]Horse, error) {
	if c == nil || c.baseURL == nil {
		return nil, ErrUnconfigured
	}
	u := *c.baseURL
	q := u.Query()
	if cursor != "" {
		q.Set("page", cursor)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create TJK request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request TJK: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("TJK returned HTTP %d", resp.StatusCode)
	}
	return c.parser.Parse(io.LimitReader(resp.Body, c.maxBody+1))
}

// HTMLTableParser parses row-oriented TJK markup. It obtains the permanent
// identity exclusively from AtId links, never from the displayed order.
type HTMLTableParser struct{}

func (HTMLTableParser) Parse(r io.Reader) ([]Horse, error) {
	doc, err := html.Parse(io.LimitReader(r, defaultMaxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("parse TJK HTML: %w", err)
	}
	var out []Horse
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			if h, ok := parseRow(n); ok {
				out = append(out, h)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if len(out) == 0 {
		return nil, fmt.Errorf("TJK HTML contains no recognizable horse rows")
	}
	return out, nil
}

func parseRow(row *html.Node) (Horse, bool) {
	var cells []string
	var number string
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "td" {
			cells = append(cells, normalizedText(n))
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					if id := atID(a.Val); id != "" {
						number = id
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(row)
	if number == "" || len(cells) < 2 {
		return Horse{}, false
	}
	// Known TJK DataRows commonly place name/race/father/mother after the ID
	// cell. Extra leading cells are tolerated, and parents remain optional.
	h := Horse{Number: number, Name: cells[1]}
	if len(cells) > 2 {
		h.Race = cells[2]
	}
	if len(cells) > 3 {
		h.Sire = cells[3]
	}
	if len(cells) > 4 {
		h.Dam = cells[4]
	}
	return h, h.Name != ""
}

func atID(href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	for key, values := range u.Query() {
		if strings.EqualFold(key, "AtId") && len(values) > 0 {
			return strings.TrimSpace(values[0])
		}
	}
	return ""
}

func normalizedText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
