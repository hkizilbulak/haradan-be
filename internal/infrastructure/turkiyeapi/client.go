package turkiyeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
)

const (
	defaultBaseURL      = "https://api.turkiyeapi.dev"
	defaultTimeout      = 15 * time.Second
	defaultMaxBodyBytes = 2 << 20
	expectedProvinces   = 81
	userAgent           = "HaradanGeoClient/1.0"
)

// Client fetches the official Turkey province/district catalog.
type Client struct {
	base    *url.URL
	http    *http.Client
	maxBody int64
}

type Config struct {
	BaseURL      string
	HTTPTimeout  time.Duration
	MaxBodyBytes int64
}

type listResponse struct {
	Status string            `json:"status"`
	Data   []provincePayload `json:"data"`
}

type provincePayload struct {
	ID        int               `json:"id"`
	Name      string            `json:"name"`
	Districts []districtPayload `json:"districts"`
}

type districtPayload struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// New constructs a Türkiye API client. Empty BaseURL uses the public host.
func New(cfg Config) (*Client, error) {
	raw := strings.TrimSpace(cfg.BaseURL)
	if raw == "" {
		raw = defaultBaseURL
	}
	base, err := url.Parse(raw)
	if err != nil || base.Scheme == "" || base.Host == "" || base.User != nil {
		return nil, fmt.Errorf("GEO_CATALOG_URL is not a valid URL")
	}
	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxBody := cfg.MaxBodyBytes
	if maxBody < 1 {
		maxBody = defaultMaxBodyBytes
	}
	httpClient := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Host != base.Host {
				return http.ErrUseLastResponse
			}
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	return &Client{base: base, http: httpClient, maxBody: maxBody}, nil
}

// FetchCatalog loads all 81 provinces with nested districts.
func (c *Client) FetchCatalog(ctx context.Context) (domaingeo.Catalog, error) {
	if c == nil || c.base == nil {
		return domaingeo.Catalog{}, apperr.DependencyUnavailable("İl ve ilçe listesi yapılandırılmamış.")
	}
	endpoint := c.base.ResolveReference(&url.URL{
		Path:     "/v1/provinces",
		RawQuery: "limit=81",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return domaingeo.Catalog{}, apperr.DependencyUnavailable("İl ve ilçe listesi alınamadı.")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	res, err := c.http.Do(req)
	if err != nil {
		return domaingeo.Catalog{}, apperr.DependencyUnavailable("İl ve ilçe listesi alınamadı.")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4<<10))
		return domaingeo.Catalog{}, apperr.DependencyUnavailable("İl ve ilçe listesi alınamadı.")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, c.maxBody+1))
	if err != nil {
		return domaingeo.Catalog{}, apperr.DependencyUnavailable("İl ve ilçe listesi alınamadı.")
	}
	if int64(len(body)) > c.maxBody {
		return domaingeo.Catalog{}, apperr.DependencyUnavailable("İl ve ilçe listesi alınamadı.")
	}

	var parsed listResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return domaingeo.Catalog{}, apperr.DependencyUnavailable("İl ve ilçe listesi okunamadı.")
	}
	if !strings.EqualFold(strings.TrimSpace(parsed.Status), "OK") {
		return domaingeo.Catalog{}, apperr.DependencyUnavailable("İl ve ilçe listesi alınamadı.")
	}
	return mapCatalog(parsed.Data)
}

func mapCatalog(rows []provincePayload) (domaingeo.Catalog, error) {
	var out domaingeo.Catalog
	if len(rows) != expectedProvinces {
		return out, apperr.DependencyUnavailable("İl ve ilçe listesi eksik veya geçersiz.")
	}
	seenPlate := make(map[int]struct{}, expectedProvinces)
	seenDistrict := make(map[int]struct{})
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if row.ID < 1 || row.ID > expectedProvinces || name == "" {
			return out, apperr.DependencyUnavailable("İl ve ilçe listesi eksik veya geçersiz.")
		}
		if _, dup := seenPlate[row.ID]; dup {
			return out, apperr.DependencyUnavailable("İl ve ilçe listesi eksik veya geçersiz.")
		}
		seenPlate[row.ID] = struct{}{}
		if len(row.Districts) < 1 {
			return out, apperr.DependencyUnavailable("İl ve ilçe listesi eksik veya geçersiz.")
		}
		provinceID := domaingeo.StableProvinceID(row.ID)
		out.Provinces = append(out.Provinces, domaingeo.Province{
			ID:        provinceID,
			Name:      name,
			SortOrder: row.ID,
			IsActive:  true,
		})
		for i, d := range row.Districts {
			dName := strings.TrimSpace(d.Name)
			if d.ID < 1 || dName == "" {
				return out, apperr.DependencyUnavailable("İl ve ilçe listesi eksik veya geçersiz.")
			}
			if _, dup := seenDistrict[d.ID]; dup {
				return out, apperr.DependencyUnavailable("İl ve ilçe listesi eksik veya geçersiz.")
			}
			seenDistrict[d.ID] = struct{}{}
			out.Districts = append(out.Districts, domaingeo.District{
				ID:         domaingeo.StableDistrictID(d.ID),
				ProvinceID: provinceID,
				Name:       dName,
				SortOrder:  i + 1,
				IsActive:   true,
			})
		}
	}
	if len(seenPlate) != expectedProvinces {
		return out, apperr.DependencyUnavailable("İl ve ilçe listesi eksik veya geçersiz.")
	}
	return out, nil
}
