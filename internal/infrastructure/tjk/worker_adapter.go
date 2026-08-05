package tjk

import (
	"context"

	apptjk "github.com/hkizilbulak/haradan-be/internal/application/tjk"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
)

type WorkerAdapter struct{ Client *Client }

func (a WorkerAdapter) FetchPage(ctx context.Context, cursor string) ([]domain.HorseInput, error) {
	page, err := a.Client.FetchPage(ctx, cursor)
	if err != nil {
		return nil, err
	}
	out := make([]domain.HorseInput, 0, len(page))
	for _, h := range page {
		out = append(out, domain.HorseInput{Number: h.Number, Name: h.Name, Race: h.Race, Sire: h.Sire, Dam: h.Dam})
	}
	return out, nil
}

var _ apptjk.PageFetcher = WorkerAdapter{}
