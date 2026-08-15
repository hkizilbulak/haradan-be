package geo

import (
	"context"
	"sync"
	"time"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
)

// CatalogSource fetches the live official province/district catalog.
type CatalogSource interface {
	FetchCatalog(ctx context.Context) (domaingeo.Catalog, error)
}

// CatalogSync refreshes local geo rows from a live catalog and serves
// subsequent reads from the database (advert FK + filters stay local).
type CatalogSync struct {
	source CatalogSource
	store  domaingeo.CatalogStore
	ttl    time.Duration

	mu     sync.Mutex
	lastOK time.Time
}

// NewCatalogSync constructs a catalog refresher. ttl <= 0 defaults to 24h.
func NewCatalogSync(source CatalogSource, store domaingeo.CatalogStore, ttl time.Duration) *CatalogSync {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &CatalogSync{source: source, store: store, ttl: ttl}
}

// Ensure refreshes the local catalog when empty or older than TTL.
// A failed refresh keeps the last good snapshot so listing is not blocked.
func (s *CatalogSync) Ensure(ctx context.Context) error {
	if s == nil || s.source == nil || s.store == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.lastOK.IsZero() && time.Since(s.lastOK) < s.ttl {
		return nil
	}
	n, err := s.store.CountActiveProvinces(ctx)
	if err != nil {
		return apperr.WrapInternal(err)
	}
	cat, fetchErr := s.source.FetchCatalog(ctx)
	if fetchErr != nil {
		if n > 0 {
			s.lastOK = time.Now()
			return nil
		}
		if ae, ok := apperr.As(fetchErr); ok {
			return ae
		}
		return apperr.DependencyUnavailable("İl ve ilçe listesi şu anda alınamıyor.")
	}
	if err := s.store.ReplaceCatalog(ctx, cat.Provinces, cat.Districts); err != nil {
		if n > 0 {
			s.lastOK = time.Now()
			return nil
		}
		return apperr.WrapInternal(err)
	}
	s.lastOK = time.Now()
	return nil
}
