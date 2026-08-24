package studfarm

import (
	"github.com/google/uuid"

	"context"

	domainstudfarm "github.com/hkizilbulak/haradan-be/internal/domain/studfarm"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

type service struct {
	repo domainstudfarm.Repository
}

// NewService constructs a stud farm application service.
func NewService(repo domainstudfarm.Repository) domainstudfarm.Service {
	return &service{
		repo: repo,
	}
}

// List returns a paginated list of stud farms.
func (s *service) List(ctx context.Context, cursor *string, limit int) (domainstudfarm.ListResult, error) {
	// Defaults
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return s.repo.List(ctx, cursor, limit)
}

// Create handles the business logic of creating a new stud farm.
func (s *service) Create(ctx context.Context, param domainstudfarm.CreateParam) (domainstudfarm.StudFarm, error) {
	return s.repo.Create(ctx, param)
}


func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}


func (s *service) AddNote(ctx context.Context, param domainstudfarm.NoteCreateParam) error {
	if param.InterviewerName == "" {
		return apperr.Validation("interviewer name is required")
	}
	if param.Notes == "" {
		return apperr.Validation("notes are required")
	}
	return s.repo.AddNote(ctx, param)
}


func (s *service) ListNotes(ctx context.Context, studFarmId uuid.UUID) ([]domainstudfarm.Note, error) {
	notes, err := s.repo.ListNotes(ctx, studFarmId)
	if err != nil {
		return nil, err
	}
	if notes == nil {
		notes = make([]domainstudfarm.Note, 0)
	}
	return notes, nil
}
