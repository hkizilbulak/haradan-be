package bankaccount

import (
	"context"

	"github.com/hkizilbulak/haradan-be/internal/domain/bank_account"
)

type Service struct {
	repo bank_account.Repository
}

func NewService(repo bank_account.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetActiveBankAccounts(ctx context.Context) ([]bank_account.BankAccount, error) {
	return s.repo.GetAll(ctx, true)
}

func (s *Service) GetAllBankAccounts(ctx context.Context) ([]bank_account.BankAccount, error) {
	return s.repo.GetAll(ctx, false)
}

func (s *Service) GetBankAccountByID(ctx context.Context, id int) (*bank_account.BankAccount, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) CreateBankAccount(ctx context.Context, p bank_account.CreateParams) (*bank_account.BankAccount, error) {
	// TODO: Add IBAN validation logic if needed
	return s.repo.Create(ctx, p)
}

func (s *Service) UpdateBankAccount(ctx context.Context, id int, p bank_account.UpdateParams) (*bank_account.BankAccount, error) {
	// TODO: Add IBAN validation logic if needed
	return s.repo.Update(ctx, id, p)
}

func (s *Service) DeleteBankAccount(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
