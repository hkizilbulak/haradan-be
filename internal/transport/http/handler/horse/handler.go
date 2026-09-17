package horse

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	apphorse "github.com/hkizilbulak/haradan-be/internal/application/horse"
	domainhorse "github.com/hkizilbulak/haradan-be/internal/domain/horse"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

var (
	tjkRowRegex  = regexp.MustCompile(`(?i)<a[^>]*QueryParameter_AtId=(\d+)[^>]*>([^<]+)</a>`)
	tjkAtIDRegex = regexp.MustCompile(`(?i)QueryParameter_AtId=(\d+)`)
	tjkAtIDCache sync.Map
	reParen      = regexp.MustCompile(`\s*[\(\[].*?[\)\]]`)
	reCoat       = regexp.MustCompile(`(?i)\s+(k\s*a|k\s*k|d\s*a|d\s*k|a\s*a|a\s*k|y\s*a|y\s*k|d\s*ö|b\s*a|kır|doru|al|yağız)$`)

	tjkHTTPClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
	}
)

func cleanHorseName(raw string) string {
	raw = strings.TrimSpace(raw)
	clean := reParen.ReplaceAllString(raw, "")
	clean = reCoat.ReplaceAllString(clean, "")
	return strings.TrimSpace(clean)
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes Horse OpenAPI operations.
type Handler struct {
	svc     *apphorse.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs a Horse HTTP handler.
func NewHandler(svc *apphorse.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// SearchHorsesForSelection handles GET /v1/horses.
func (h *Handler) SearchHorsesForSelection(c *gin.Context, params generated.SearchHorsesForSelectionParams) {
	var limit *int
	if params.Limit != nil {
		v := int(*params.Limit)
		limit = &v
	}
	items, err := h.svc.SearchForSelection(c.Request.Context(), params.Q, params.TjkNumber, limit)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.HorseSelectionListResponse{Items: mapSelection(items)})
}

// GetHorsePublicDetail handles GET /v1/horses/{horseId}.
func (h *Handler) GetHorsePublicDetail(c *gin.Context, horseID uuid.UUID) {
	out, err := h.svc.GetPublicDetail(c.Request.Context(), horseID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapPublicDetail(out))
}

func (h *Handler) resolveHorseAtID(ctx context.Context, clean string) (string, string) {
	if isNumeric(clean) {
		return clean, "https://www.tjk.org/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri?1=1&QueryParameter_AtId=" + url.QueryEscape(clean)
	}

	cacheKey := strings.ToLower(clean)
	if val, ok := tjkAtIDCache.Load(cacheKey); ok {
		if cachedID, ok := val.(string); ok && cachedID != "" {
			return cachedID, "https://www.tjk.org/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri?1=1&QueryParameter_AtId=" + url.QueryEscape(cachedID)
		}
	}

	// 1. Check local DB
	if h.svc != nil {
		limit := 10
		items, err := h.svc.SearchForSelection(ctx, &clean, nil, &limit)
		if err == nil && len(items) > 0 {
			for _, it := range items {
				if strings.EqualFold(strings.TrimSpace(it.OriginalName), clean) && it.TJKNumber != "" {
					tjkAtIDCache.Store(cacheKey, it.TJKNumber)
					return it.TJKNumber, "https://www.tjk.org/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri?1=1&QueryParameter_AtId=" + url.QueryEscape(it.TJKNumber)
				}
			}
			if items[0].TJKNumber != "" {
				tjkAtIDCache.Store(cacheKey, items[0].TJKNumber)
				return items[0].TJKNumber, "https://www.tjk.org/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri?1=1&QueryParameter_AtId=" + url.QueryEscape(items[0].TJKNumber)
			}
		}
	}

	// 2. Query TJK live with QueryParameter_OLDUFLG=on via fast DataRows AJAX endpoint
	tjkSearchURL := "https://www.tjk.org/TR/YarisSever/Query/Page/Atlar?1=1&QueryParameter_AtIsmi=" + url.QueryEscape(clean) + "&QueryParameter_OLDUFLG=on"
	dataRowsURL := "https://www.tjk.org/TR/YarisSever/Query/DataRows/Atlar?QueryParameter_AtIsmi=" + url.QueryEscape(clean) + "&QueryParameter_OLDUFLG=on&X-Requested-With=XMLHttpRequest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dataRowsURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		resp, err := tjkHTTPClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))

			// Try finding best matching row
			rows := tjkRowRegex.FindAllSubmatch(body, -1)
			var matchedID string
			for _, r := range rows {
				if len(r) > 2 {
					id := string(r[1])
					linkText := strings.TrimSpace(string(r[2]))
					cleanLinkText := strings.TrimSuffix(linkText, "(Öldü)")
					cleanLinkText = strings.TrimSpace(cleanLinkText)
					if strings.EqualFold(cleanLinkText, clean) {
						matchedID = id
						break
					}
				}
			}
			// If no exact name row, take first match
			if matchedID == "" && len(rows) > 0 && len(rows[0]) > 1 {
				matchedID = string(rows[0][1])
			}
			// Fallback to simple regex if row regex missed
			if matchedID == "" {
				single := tjkAtIDRegex.FindSubmatch(body)
				if len(single) > 1 {
					matchedID = string(single[1])
				}
			}

			if matchedID != "" {
				tjkAtIDCache.Store(cacheKey, matchedID)
				return matchedID, "https://www.tjk.org/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri?1=1&QueryParameter_AtId=" + url.QueryEscape(matchedID)
			}
		}
	}

	return "", tjkSearchURL
}

// RedirectToTJK handles GET /v1/horses/tjk-redirect?name=...&atId=...
func (h *Handler) RedirectToTJK(c *gin.Context) {
	atID := strings.TrimSpace(c.Query("atId"))
	if atID != "" {
		c.Redirect(http.StatusFound, "https://www.tjk.org/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri?1=1&QueryParameter_AtId="+url.QueryEscape(atID))
		return
	}

	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		c.Redirect(http.StatusFound, "https://www.tjk.org/TR/YarisSever/Query/Page/Atlar?QueryParameter_OLDUFLG=on")
		return
	}

	clean := cleanHorseName(name)
	if clean == "" {
		clean = name
	}

	_, targetURL := h.resolveHorseAtID(c.Request.Context(), clean)
	c.Redirect(http.StatusFound, targetURL)
}

// ResolveTJK handles GET /v1/horses/tjk-resolve?name=...&atId=...
func (h *Handler) ResolveTJK(c *gin.Context) {
	atID := strings.TrimSpace(c.Query("atId"))
	if atID != "" {
		c.JSON(http.StatusOK, gin.H{
			"atId": atID,
			"url":  "https://www.tjk.org/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri?1=1&QueryParameter_AtId=" + atID,
		})
		return
	}

	name := strings.TrimSpace(c.Query("name"))
	clean := cleanHorseName(name)
	if clean == "" {
		clean = name
	}

	foundID, targetURL := h.resolveHorseAtID(c.Request.Context(), clean)
	c.JSON(http.StatusOK, gin.H{
		"atId": foundID,
		"url":  targetURL,
	})
}

// RegisterRoutes registers non-OpenAPI custom routes.
func (h *Handler) RegisterRoutes(rg gin.IRouter) {
	rg.GET("/v1/tjk/redirect", h.RedirectToTJK)
	rg.GET("/v1/tjk/resolve", h.ResolveTJK)
}

func mapSelection(items []domainhorse.SelectionProjection) []generated.HorseSelectionItem {
	out := make([]generated.HorseSelectionItem, 0, len(items))
	for _, item := range items {
		out = append(out, generated.HorseSelectionItem{
			Id:           openapi_types.UUID(item.ID),
			OriginalName: item.OriginalName,
			TjkNumber:    item.TJKNumber,
			BirthYear:    item.BirthYear,
			SireName:     item.SireName,
			DamName:      item.DamName,
		})
	}
	return out
}

func mapPublicDetail(out domainhorse.PublicDetail) generated.HorsePublicDetailResponse {
	detail := map[string]interface{}{}
	if len(out.Detail) > 0 {
		_ = json.Unmarshal(out.Detail, &detail)
		if detail == nil {
			detail = map[string]interface{}{}
		}
	}
	return generated.HorsePublicDetailResponse{
		Id:           openapi_types.UUID(out.ID),
		OriginalName: out.OriginalName,
		TjkNumber:    out.TJKNumber,
		BirthYear:    out.BirthYear,
		SireName:     out.SireName,
		DamName:      out.DamName,
		Breed:        out.Breed,
		Gender:       out.Gender,
		Coat:         out.Coat,
		Detail:       detail,
	}
}
