-- +goose Up
ALTER TABLE hrd_users ADD COLUMN channel varchar(32) NOT NULL DEFAULT 'EMAIL';
CREATE INDEX hrd_users_channel_idx ON hrd_users (channel);

-- +goose Down
DROP INDEX hrd_users_channel_idx;
ALTER TABLE hrd_users DROP COLUMN channel;
