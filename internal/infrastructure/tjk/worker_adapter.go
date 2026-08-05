package tjk

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	apptjk "github.com/hkizilbulak/haradan-be/internal/application/tjk"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
)

// WorkerAdapter adapts the TJK HTTP client to the application PageFetcher.
// Per-horse detail/pedigree/sibling enrichment is best-effort: a failure for
// one horse never aborts the bulk page.
type WorkerAdapter struct{ Client *Client }

func (a WorkerAdapter) FetchPage(ctx context.Context, cursor string) ([]domain.HorseInput, error) {
	page, err := a.Client.FetchPage(ctx, cursor)
	if err != nil {
		return nil, err
	}
	out := make([]domain.HorseInput, 0, len(page))
	for _, h := range page {
		in := domain.HorseInput{
			Number: h.Number,
			Name:   h.Name,
			Race:   h.Race,
			Sire:   h.Sire,
			Dam:    h.Dam,
		}
		out = append(out, a.enrichHorse(ctx, in))
	}
	return out, nil
}

func (a WorkerAdapter) enrichHorse(ctx context.Context, in domain.HorseInput) domain.HorseInput {
	if a.Client == nil || strings.TrimSpace(in.Number) == "" {
		return in
	}

	var pedigree []PedigreeEntry
	var siblings []Sibling
	var stats []RaceStatistic

	if d, err := a.Client.FetchDetail(ctx, in.Number); err == nil {
		stats = d.Statistics
		if strings.TrimSpace(in.Sire) == "" && strings.TrimSpace(d.Sire) != "" {
			in.Sire = strings.TrimSpace(d.Sire)
		}
		if strings.TrimSpace(in.Dam) == "" && strings.TrimSpace(d.Dam) != "" {
			in.Dam = strings.TrimSpace(d.Dam)
		}
		if y := birthYearFromDate(d.BirthDate); y != nil {
			in.BirthYear = y
		}
		if g := genderFromAgeText(d.AgeText); g != "" {
			in.Gender = &g
		}
	}

	if p, err := a.Client.FetchPedigree(ctx, in.Number); err == nil {
		pedigree = p
	}
	if s, err := a.Client.FetchSiblings(ctx, in.Number); err == nil {
		siblings = s
	}

	doc := BuildDetailDocument(pedigree, siblings, stats)
	if len(doc.Pedigree) == 0 && len(doc.Siblings) == 0 && len(doc.Statistics) == 0 {
		return in
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return in
	}
	in.Detail = raw
	return in
}

func birthYearFromDate(value string) *int {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	// Legacy TJK birth dates use dd.MM.yyyy.
	if t, err := time.Parse("02.01.2006", value); err == nil {
		y := t.Year()
		if y >= 1800 && y <= 2200 {
			return &y
		}
		return nil
	}
	// Fallback: trailing 4-digit year.
	if len(value) >= 4 {
		if y, err := strconv.Atoi(value[len(value)-4:]); err == nil && y >= 1800 && y <= 2200 {
			return &y
		}
	}
	return nil
}

func genderFromAgeText(ageText string) string {
	letters := make([]rune, 0, 8)
	for _, r := range ageText {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			letters = append(letters, r)
		}
	}
	if len(letters) == 0 {
		return ""
	}
	return string(letters[len(letters)-1])
}

var _ apptjk.PageFetcher = WorkerAdapter{}
