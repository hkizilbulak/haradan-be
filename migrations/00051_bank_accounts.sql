-- +goose Up
-- +goose StatementBegin
CREATE TABLE hrd_bank_accounts (
    id SERIAL PRIMARY KEY,
    bank_name VARCHAR(100) NOT NULL,
    account_holder VARCHAR(255) NOT NULL,
    iban VARCHAR(34) NOT NULL,
    branch_name VARCHAR(100),
    account_number VARCHAR(50),
    is_active BOOLEAN NOT NULL DEFAULT true,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE hrd_bank_accounts;
-- +goose StatementEnd
