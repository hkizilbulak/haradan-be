-- +goose Up
ALTER TABLE hrd_adverts ADD COLUMN address text NULL;

-- +goose Down
ALTER TABLE hrd_adverts DROP COLUMN address;
