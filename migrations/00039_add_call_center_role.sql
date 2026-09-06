-- +goose Up
ALTER TABLE hrd_users DROP CONSTRAINT hrd_users_role_check;
ALTER TABLE hrd_users ADD CONSTRAINT hrd_users_role_check CHECK (role IN ('user', 'admin', 'CALL_CENTER'));

-- +goose Down
ALTER TABLE hrd_users DROP CONSTRAINT hrd_users_role_check;
ALTER TABLE hrd_users ADD CONSTRAINT hrd_users_role_check CHECK (role IN ('user', 'admin'));
