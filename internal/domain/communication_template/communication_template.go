package communication_template

import (
	"context"
	"time"
)

type Template struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Channel   string    `json:"channel"`
	Content   string    `json:"content"`
	Subject   *string   `json:"subject"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateParams struct {
	Title   string
	Channel string
	Content string
	Subject *string
}

type UpdateParams struct {
	Title   string
	Channel string
	Content string
	Subject *string
}

type Repository interface {
	GetAll(ctx context.Context) ([]Template, error)
	Create(ctx context.Context, p CreateParams) (*Template, error)
	Update(ctx context.Context, id string, p UpdateParams) (*Template, error)
	Delete(ctx context.Context, id string) error
}
