package horse

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Horse is the canonical TJK-backed horse entity.
type Horse struct {
	ID              uuid.UUID
	TJKNumber       string
	OriginalName    string
	NameNormalized  string
	BirthYear       *int
	SireName        *string
	DamName         *string
	Breed           *string
	Gender          *string
	Coat            *string
	Detail          json.RawMessage
	LastSyncedAt    *time.Time
	LastSeenAt      *time.Time
	SourceUpdatedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SelectionProjection is the safe list/search projection for FE selection.
type SelectionProjection struct {
	ID           uuid.UUID
	OriginalName string
	TJKNumber    string
	BirthYear    *int
	SireName     *string
	DamName      *string
}

// PublicDetail is the public detail projection without sync/raw internals.
type PublicDetail struct {
	ID           uuid.UUID
	OriginalName string
	TJKNumber    string
	BirthYear    *int
	SireName     *string
	DamName      *string
	Breed        *string
	Gender       *string
	Coat         *string
	Detail       json.RawMessage
}

// Repository reads horse reference data from PostgreSQL.
type Repository interface {
	FindByID(ctx context.Context, id uuid.UUID) (Horse, error)
	FindByTJKNumber(ctx context.Context, tjkNumber string) (Horse, error)
	SearchByNormalizedPrefix(ctx context.Context, prefix string, limit int) ([]Horse, error)
}
