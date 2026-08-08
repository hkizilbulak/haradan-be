-- +goose Up
ALTER TABLE hrd_campaigns
    ADD COLUMN email_provider_template_id varchar(128) NULL;

-- A QUEUED TJK run may be cancelled before a worker starts it. Such a run is
-- terminal with completed_at set while started_at correctly remains NULL.
ALTER TABLE hrd_tjk_sync_runs
    DROP CONSTRAINT hrd_tjk_sync_runs_time_order_check;

ALTER TABLE hrd_tjk_sync_runs
    ADD CONSTRAINT hrd_tjk_sync_runs_time_order_check CHECK (
        (started_at IS NULL OR started_at >= created_at)
        AND (
            completed_at IS NULL
            OR (status = 'CANCELLED' AND started_at IS NULL)
            OR (started_at IS NOT NULL AND completed_at >= started_at)
        )
        AND (cancelled_at IS NULL OR cancelled_at >= created_at)
    );

-- +goose Down
ALTER TABLE hrd_tjk_sync_runs
    DROP CONSTRAINT hrd_tjk_sync_runs_time_order_check;

ALTER TABLE hrd_tjk_sync_runs
    ADD CONSTRAINT hrd_tjk_sync_runs_time_order_check CHECK (
        (started_at IS NULL OR started_at >= created_at)
        AND (completed_at IS NULL OR (started_at IS NOT NULL AND completed_at >= started_at))
        AND (cancelled_at IS NULL OR cancelled_at >= created_at)
    );

ALTER TABLE hrd_campaigns
    DROP COLUMN IF EXISTS email_provider_template_id;
