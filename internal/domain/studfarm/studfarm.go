package studfarm

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// StudFarm represents the base haralar entity.
type StudFarm struct {
	ID        uuid.UUID
	FirstName string
	LastName  string
	Email     string
	Phone     *string
	Location  *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// StudFarmListItem represents the projection for the BO list view.
type StudFarmListItem struct {
	StudFarm
	LatestInterviewDate *time.Time
	InterviewerName     *string
	InterviewNotesURL   *string
	InterviewCount      int
}

// ListResult contains the paginated items and cursor info.
type ListResult struct {
	Items      []StudFarmListItem
	NextCursor *string
	HasMore    bool
}

// CreateParam holds data to create a new stud farm.

// NoteCreateParam holds data to create a new stud farm note.

// Note represents a stud farm note.
type Note struct {
	ID              uuid.UUID
	StudFarmID      uuid.UUID
	InterviewerName string
	InterviewDate   time.Time
	Notes           string
	CreatedAt       time.Time
}

type NoteCreateParam struct {
	StudFarmID      uuid.UUID
	InterviewerName string
	InterviewDate   time.Time
	Notes           string
}

type CreateParam struct {
	FirstName string
	LastName  string
	Email     string
	Phone     *string
	Location  *string
}

// Repository defines data access for stud farms.
type Repository interface {
	List(ctx context.Context, cursor *string, limit int) (ListResult, error)
	Create(ctx context.Context, param CreateParam) (StudFarm, error)
	Delete(ctx context.Context, id uuid.UUID) error
	AddNote(ctx context.Context, param NoteCreateParam) error
	ListNotes(ctx context.Context, studFarmId uuid.UUID) ([]Note, error)
}

// Service defines the business logic for stud farms.
type Service interface {
	List(ctx context.Context, cursor *string, limit int) (ListResult, error)
	Create(ctx context.Context, param CreateParam) (StudFarm, error)
	Delete(ctx context.Context, id uuid.UUID) error
	AddNote(ctx context.Context, param NoteCreateParam) error
	ListNotes(ctx context.Context, studFarmId uuid.UUID) ([]Note, error)
}
