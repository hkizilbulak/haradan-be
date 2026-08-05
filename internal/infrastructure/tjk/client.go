// Package tjk implements the bounded, redirect-free HTTP adapter for the
// public TJK horse pages. Paths and query parameters mirror the legacy
// Haradan TjkUrlType contract relative to a configurable base host.
package tjk

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

const (
	defaultMaxBodyBytes int64 = 2 << 20
	defaultUserAgent          = "HaradanTJKClient/1.0"

	pathBulkSummary = "/TR/YarisSever/Query/DataRows/Atlar"
	pathDetail      = "/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri"
	pathPedigree    = "/TR/YarisSever/Query/Pedigri/Pedigri"
	pathSibling     = "/TR/YarisSever/Query/Kardes/Kardes"
)

var ErrUnconfigured = permanentErr("tjk adapter is not configured", 0)

// Horse is a bulk-summary row from DataRows/Atlar.
type Horse struct {
	Number string
	Name   string
	Race   string
	Sire   string
	Dam    string
}

// Detail holds typed fields and race statistics from AtKosuBilgileri.
type Detail struct {
	Name          string
	AgeText       string
	BirthDate     string
	HandicapPoint string
	Sire          string
	Dam           string
	Owner         string
	Grower        string
	Statistics    []RaceStatistic
}

// RaceStatistic is one statistics table row.
type RaceStatistic struct {
	YearLabel string
	RaceCount string
	First     string
	Second    string
	Third     string
	Fourth    string
	Fifth     string
	Earning   string
}

// PedigreeEntry is one father/mother pair from the Pedigri table.
type PedigreeEntry struct {
	Father string
	Mother string
}

// Sibling is one Kardes table row.
type Sibling struct {
	Name       string
	FatherName string
	RaceCount  string
	First      string
	Second     string
	Third      string
	Fourth     string
	Earning    string
}

// DetailDocument is the controlled subset of horse.detail JSONB keys that this
// adapter can populate from pedigree/sibling/statistics responses.
type DetailDocument struct {
	Pedigree   []PedigreeEntry `json:"pedigree,omitempty"`
	Siblings   []Sibling       `json:"siblings,omitempty"`
	Statistics []RaceStatistic `json:"statistics,omitempty"`
}

// BuildDetailDocument maps supported client results into the horse detail object.
func BuildDetailDocument(pedigree []PedigreeEntry, siblings []Sibling, stats []RaceStatistic) DetailDocument {
	return DetailDocument{
		Pedigree:   pedigree,
		Siblings:   siblings,
		Statistics: stats,
	}
}

// Config configures the TJK HTTP client. BaseURL is the host origin
// (for example https://www.tjk.org); legacy paths are appended.
type Config struct {
	BaseURL      string
	HTTPTimeout  time.Duration
	MaxBodyBytes int64
	UserAgent    string
	HTTPClient   *http.Client
}

// Client fetches and parses TJK pages without following redirects.
type Client struct {
	baseURL   *url.URL
	http      *http.Client
	maxBody   int64
	userAgent string
}

// NewClient validates configuration and returns a redirect-blocking client.
func NewClient(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, ErrUnconfigured
	}
	u, err := url.Parse(strings.TrimSpace(cfg.BaseURL))
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
		return nil, permanentErr("invalid TJK base URL", 0)
	}
	// Keep scheme+host(+port) only; paths are always legacy-absolute.
	u.Path, u.RawPath, u.RawQuery, u.Fragment = "", "", "", ""
	if cfg.HTTPTimeout <= 0 {
		return nil, permanentErr("TJK HTTP timeout must be greater than zero", 0)
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMaxBodyBytes
	}
	ua := strings.TrimSpace(cfg.UserAgent)
	if ua == "" {
		ua = defaultUserAgent
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
	return &Client{baseURL: u, http: &copyClient, maxBody: cfg.MaxBodyBytes, userAgent: ua}, nil
}

// FetchPage requests legacy bulk summary DataRows/Atlar for a 0-based PageNumber.
// An empty cursor is treated as page 0. Empty result means end of listing.
func (c *Client) FetchPage(ctx context.Context, cursor string) ([]Horse, error) {
	page := strings.TrimSpace(cursor)
	if page == "" {
		page = "0"
	}
	q := url.Values{}
	q.Set("PageNumber", page)
	q.Set("Sort", "AtIsmi")
	q.Set("X-Requested-With", "XMLHttpRequest")
	body, err := c.get(ctx, pathBulkSummary, q)
	if err != nil {
		return nil, err
	}
	return parseBulkSummary(body)
}

// FetchDetail requests AtKosuBilgileri for the given AtId.
func (c *Client) FetchDetail(ctx context.Context, atID string) (Detail, error) {
	atID = strings.TrimSpace(atID)
	if atID == "" {
		return Detail{}, permanentErr("TJK AtId is required", 0)
	}
	q := url.Values{}
	q.Set("1", "1")
	q.Set("QueryParameter_AtId", atID)
	body, err := c.get(ctx, pathDetail, q)
	if err != nil {
		return Detail{}, err
	}
	return parseDetail(body)
}

// FetchPedigree requests Pedigri for the given Atkodu.
func (c *Client) FetchPedigree(ctx context.Context, atID string) ([]PedigreeEntry, error) {
	atID = strings.TrimSpace(atID)
	if atID == "" {
		return nil, permanentErr("TJK Atkodu is required", 0)
	}
	q := url.Values{}
	q.Set("Atkodu", atID)
	body, err := c.get(ctx, pathPedigree, q)
	if err != nil {
		return nil, err
	}
	return parsePedigree(body)
}

// FetchSiblings requests Kardes for the given Atkodu.
func (c *Client) FetchSiblings(ctx context.Context, atID string) ([]Sibling, error) {
	atID = strings.TrimSpace(atID)
	if atID == "" {
		return nil, permanentErr("TJK Atkodu is required", 0)
	}
	q := url.Values{}
	q.Set("Atkodu", atID)
	body, err := c.get(ctx, pathSibling, q)
	if err != nil {
		return nil, err
	}
	return parseSiblings(body)
}

func (c *Client) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	if c == nil || c.baseURL == nil {
		return nil, ErrUnconfigured
	}
	ref := &url.URL{Path: path, RawQuery: query.Encode()}
	reqURL := c.baseURL.ResolveReference(ref).String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, permanentErr("create TJK request failed", 0)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, mapTransportError(err)
	}
	defer resp.Body.Close()

	if err := mapHTTPStatus(resp.StatusCode); err != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, c.maxBody))
		return nil, err
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBody+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, mapTransportError(err)
	}
	if int64(len(raw)) > c.maxBody {
		return nil, permanentErr("TJK response exceeds body limit", 0)
	}
	decoded, err := charset.NewReader(bytes.NewReader(raw), resp.Header.Get("Content-Type"))
	if err != nil {
		return raw, nil
	}
	data, err := io.ReadAll(decoded)
	if err != nil {
		return raw, nil
	}
	return data, nil
}
