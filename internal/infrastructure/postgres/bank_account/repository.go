package bankaccount

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hkizilbulak/haradan-be/internal/domain/bank_account"
)

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) bank_account.Repository {
	return &repository{pool: pool}
}

func (r *repository) GetAll(ctx context.Context, onlyActive bool) ([]bank_account.BankAccount, error) {
	query := `
		SELECT id, bank_name, account_holder, iban, branch_name, account_number, is_active, display_order, created_at, updated_at
		FROM hrd_bank_accounts
	`
	args := []interface{}{}

	if onlyActive {
		query += " WHERE is_active = true"
	}

	query += " ORDER BY display_order ASC, created_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []bank_account.BankAccount
	for rows.Next() {
		var a bank_account.BankAccount
		if err := rows.Scan(
			&a.ID,
			&a.BankName,
			&a.AccountHolder,
			&a.IBAN,
			&a.BranchName,
			&a.AccountNumber,
			&a.IsActive,
			&a.DisplayOrder,
			&a.CreatedAt,
			&a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (r *repository) GetByID(ctx context.Context, id int) (*bank_account.BankAccount, error) {
	query := `
		SELECT id, bank_name, account_holder, iban, branch_name, account_number, is_active, display_order, created_at, updated_at
		FROM hrd_bank_accounts
		WHERE id = $1
	`
	var a bank_account.BankAccount
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&a.ID,
		&a.BankName,
		&a.AccountHolder,
		&a.IBAN,
		&a.BranchName,
		&a.AccountNumber,
		&a.IsActive,
		&a.DisplayOrder,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Or an appropriate domain error
		}
		return nil, err
	}
	return &a, nil
}

func (r *repository) Create(ctx context.Context, p bank_account.CreateParams) (*bank_account.BankAccount, error) {
	query := `
		INSERT INTO hrd_bank_accounts (bank_name, account_holder, iban, branch_name, account_number, is_active, display_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, bank_name, account_holder, iban, branch_name, account_number, is_active, display_order, created_at, updated_at
	`
	var a bank_account.BankAccount
	err := r.pool.QueryRow(ctx, query, p.BankName, p.AccountHolder, p.IBAN, p.BranchName, p.AccountNumber, p.IsActive, p.DisplayOrder).Scan(
		&a.ID,
		&a.BankName,
		&a.AccountHolder,
		&a.IBAN,
		&a.BranchName,
		&a.AccountNumber,
		&a.IsActive,
		&a.DisplayOrder,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *repository) Update(ctx context.Context, id int, p bank_account.UpdateParams) (*bank_account.BankAccount, error) {
	query := `
		UPDATE hrd_bank_accounts
		SET bank_name = $1, account_holder = $2, iban = $3, branch_name = $4, account_number = $5, is_active = $6, display_order = $7, updated_at = NOW()
		WHERE id = $8
		RETURNING id, bank_name, account_holder, iban, branch_name, account_number, is_active, display_order, created_at, updated_at
	`
	var a bank_account.BankAccount
	err := r.pool.QueryRow(ctx, query, p.BankName, p.AccountHolder, p.IBAN, p.BranchName, p.AccountNumber, p.IsActive, p.DisplayOrder, id).Scan(
		&a.ID,
		&a.BankName,
		&a.AccountHolder,
		&a.IBAN,
		&a.BranchName,
		&a.AccountNumber,
		&a.IsActive,
		&a.DisplayOrder,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Or appropriate error
		}
		return nil, err
	}
	return &a, nil
}

func (r *repository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM hrd_bank_accounts WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}
