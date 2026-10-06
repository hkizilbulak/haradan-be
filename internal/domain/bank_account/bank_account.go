package bank_account

import (
	"context"
	"time"
)

type BankAccount struct {
	ID            int       `json:"id"`
	BankName      string    `json:"bank_name"`
	AccountHolder string    `json:"account_holder"`
	IBAN          string    `json:"iban"`
	BranchName    *string   `json:"branch_name"`
	AccountNumber *string   `json:"account_number"`
	IsActive      bool      `json:"is_active"`
	DisplayOrder  int       `json:"display_order"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateParams struct {
	BankName      string
	AccountHolder string
	IBAN          string
	BranchName    *string
	AccountNumber *string
	IsActive      bool
	DisplayOrder  int
}

type UpdateParams struct {
	BankName      string
	AccountHolder string
	IBAN          string
	BranchName    *string
	AccountNumber *string
	IsActive      bool
	DisplayOrder  int
}

type Repository interface {
	GetAll(ctx context.Context, onlyActive bool) ([]BankAccount, error)
	GetByID(ctx context.Context, id int) (*BankAccount, error)
	Create(ctx context.Context, p CreateParams) (*BankAccount, error)
	Update(ctx context.Context, id int, p UpdateParams) (*BankAccount, error)
	Delete(ctx context.Context, id int) error
}
