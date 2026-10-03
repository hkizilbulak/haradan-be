-- +goose Up
ALTER TABLE hrd_adverts ADD COLUMN video_url text NULL;

-- +goose Down
ALTER TABLE hrd_adverts DROP COLUMN video_url;
