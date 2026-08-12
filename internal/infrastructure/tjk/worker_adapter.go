package tjk

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	apptjk "github.com/hkizilbulak/haradan-be/internal/application/tjk"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
)

// WorkerAdapter adapts the TJK HTTP client to the application PageFetcher.
// Per-horse detail/pedigree/sibling enrichment is best-effort: a failure for
// one horse never aborts the bulk page.
type WorkerAdapter struct{ Client *Client }

const enrichmentConcurrency = 8

func (a WorkerAdapter) FetchPage(ctx context.Context, cursor string) (domain.PageResult, error) {
	page, err := a.Client.FetchPage(ctx, cursor)
	if err != nil {
		return domain.PageResult{}, err
	}
	if page.EndOfSource {
		return domain.PageResult{EndOfSource: true, SourceTotal: page.SourceTotal}, nil
	}
	out := make([]domain.HorseInput, len(page.Horses))
	sem := make(chan struct{}, enrichmentConcurrency)
	var wg sync.WaitGroup
	for i, h := range page.Horses {
		in := domain.HorseInput{
			Number: h.Number,
			Name:   h.Name,
			Race:   h.Race,
			Sire:   h.Sire,
			Dam:    h.Dam,
		}
		out[i] = in
		wg.Add(1)
		go func(index int, input domain.HorseInput) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			out[index] = a.enrichHorse(ctx, input)
		}(i, in)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return domain.PageResult{}, err
	}
	return domain.PageResult{
		Horses: out, Fingerprint: page.Fingerprint,
		SourceTotal: page.SourceTotal, SkippedCount: page.SkippedCount,
	}, nil
}

func (a WorkerAdapter) enrichHorse(ctx context.Context, in domain.HorseInput) domain.HorseInput {
	if a.Client == nil || strings.TrimSpace(in.Number) == "" {
		return in
	}

	var doc DetailDocument

	if d, err := a.Client.FetchDetail(ctx, in.Number); err == nil {
		doc.Profile = &DetailProfile{
			SourceName: d.Name, AgeText: d.AgeText, BirthDate: d.BirthDate,
			HandicapPoint: d.HandicapPoint, MaidenSire: d.MaidenSire,
			Owner: d.Owner, Grower: d.Grower, Earning: d.Earning,
		}
		doc.Statistics = &d.Statistics
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
		if coat := coatFromAgeText(d.AgeText); coat != "" {
			in.Coat = &coat
		}
	} else {
		in.EnrichmentIssues = append(in.EnrichmentIssues, domain.EnrichmentIssue{
			Component: "detail", Message: "TJK horse detail could not be retrieved",
		})
	}

	if p, err := a.Client.FetchPedigree(ctx, in.Number); err == nil {
		doc.Pedigree = &p
	} else {
		in.EnrichmentIssues = append(in.EnrichmentIssues, domain.EnrichmentIssue{
			Component: "pedigree", Message: "TJK horse pedigree could not be retrieved",
		})
	}
	if s, err := a.Client.FetchSiblings(ctx, in.Number); err == nil {
		doc.Siblings = &s
	} else {
		in.EnrichmentIssues = append(in.EnrichmentIssues, domain.EnrichmentIssue{
			Component: "siblings", Message: "TJK horse siblings could not be retrieved",
		})
	}
	if m, err := a.Client.FetchMating(ctx, in.Number); err == nil {
		doc.Mating = &m
	} else {
		in.EnrichmentIssues = append(in.EnrichmentIssues, domain.EnrichmentIssue{
			Component: "mating", Message: "TJK horse mating statistics could not be retrieved",
		})
	}
	if o, err := a.Client.FetchOffspring(ctx, in.Number); err == nil {
		doc.Offspring = &o
	} else {
		in.EnrichmentIssues = append(in.EnrichmentIssues, domain.EnrichmentIssue{
			Component: "offspring", Message: "TJK horse offspring statistics could not be retrieved",
		})
	}

	if doc.Profile == nil && doc.Pedigree == nil && doc.Siblings == nil && doc.Statistics == nil && doc.Mating == nil && doc.Offspring == nil {
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

func coatFromAgeText(ageText string) string {
	letters := make([]rune, 0, 8)
	for _, r := range ageText {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			letters = append(letters, r)
		}
	}
	if len(letters) < 2 {
		return ""
	}
	return string(letters[len(letters)-2])
}

var _ apptjk.PageFetcher = WorkerAdapter{}
