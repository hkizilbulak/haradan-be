-- +goose Up
-- Add processed_count to hrd_background_jobs for execution metrics tracking.
ALTER TABLE hrd_background_jobs
    ADD COLUMN processed_count integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE hrd_background_jobs
    DROP COLUMN processed_count;
