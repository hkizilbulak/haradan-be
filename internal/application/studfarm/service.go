package studfarm

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainstudfarm "github.com/hkizilbulak/haradan-be/internal/domain/studfarm"
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
	if strings.TrimSpace(param.FirstName) == "" {
		return domainstudfarm.StudFarm{}, apperr.Validation("first name is required")
	}
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

func (s *service) DeleteNote(ctx context.Context, studFarmId uuid.UUID, noteId uuid.UUID) error {
	return s.repo.DeleteNote(ctx, studFarmId, noteId)
}

func (s *service) UpdateNote(ctx context.Context, studFarmId uuid.UUID, noteId uuid.UUID, param domainstudfarm.NoteCreateParam) error {
	return s.repo.UpdateNote(ctx, studFarmId, noteId, param)
}

func (s *service) Update(ctx context.Context, id uuid.UUID, param domainstudfarm.CreateParam) error {
	if strings.TrimSpace(param.FirstName) == "" {
		return apperr.Validation("first name is required")
	}
	return s.repo.Update(ctx, id, param)
}
