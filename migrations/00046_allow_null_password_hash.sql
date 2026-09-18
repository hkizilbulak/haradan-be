-- +goose Up
ALTER TABLE hrd_users ALTER COLUMN password_hash DROP NOT NULL;

-- +goose Down
ALTER TABLE hrd_users ALTER COLUMN password_hash SET NOT NULL;
