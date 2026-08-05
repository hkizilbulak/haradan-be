-- +goose Up
-- Replace global all-status dedup uniqueness with an active/success partial unique.
-- FAILED/DEAD/CANCELLED rows may share a deduplication_key so work can be re-enqueued.
-- Keep hrd_background_jobs_scheduled_occurrence_key from 00012 (definition-backed).

DROP INDEX hrd_background_jobs_deduplication_key_key;

CREATE UNIQUE INDEX hrd_background_jobs_active_dedup_key
    ON hrd_background_jobs (deduplication_key)
    WHERE deduplication_key IS NOT NULL
      AND status IN ('QUEUED', 'LEASED', 'SUCCEEDED');

-- +goose Down
DROP INDEX hrd_background_jobs_active_dedup_key;

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM (
            SELECT deduplication_key
            FROM hrd_background_jobs
            WHERE deduplication_key IS NOT NULL
            GROUP BY deduplication_key
            HAVING COUNT(*) > 1
        ) dups
    ) THEN
        RAISE EXCEPTION
            'cannot recreate hrd_background_jobs_deduplication_key_key: duplicate non-null deduplication_key values exist across statuses';
    END IF;
END $$;
-- +goose StatementEnd

CREATE UNIQUE INDEX hrd_background_jobs_deduplication_key_key
    ON hrd_background_jobs (deduplication_key)
    WHERE deduplication_key IS NOT NULL;
