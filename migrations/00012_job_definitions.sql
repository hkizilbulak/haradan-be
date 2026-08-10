-- +goose Up
-- Scheduled job definitions for BO-managed worker cron (Kartezya-aligned API surface).

CREATE TABLE hrd_job_definitions (
    id uuid NOT NULL,
    job_key varchar(100) NOT NULL,
    name varchar(255) NOT NULL,
    description text NULL,
    job_type varchar(64) NOT NULL,
    cron_expression varchar(100) NOT NULL,
    is_active boolean NOT NULL DEFAULT false,
    timeout_seconds integer NOT NULL DEFAULT 3600,
    default_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    supports_reference_date boolean NOT NULL DEFAULT false,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_job_definitions_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_job_definitions_job_key_key UNIQUE (job_key),
    CONSTRAINT hrd_job_definitions_job_key_not_blank_check
        CHECK (btrim(job_key) <> ''),
    CONSTRAINT hrd_job_definitions_name_not_blank_check
        CHECK (btrim(name) <> ''),
    CONSTRAINT hrd_job_definitions_cron_not_blank_check
        CHECK (btrim(cron_expression) <> ''),
    CONSTRAINT hrd_job_definitions_job_type_check CHECK (job_type IN (
        'TJK_SYNC',
        'PACKAGE_EXPIRY_SCAN',
        'MEDIA_RECONCILE'
    )),
    CONSTRAINT hrd_job_definitions_timeout_positive_check
        CHECK (timeout_seconds > 0),
    CONSTRAINT hrd_job_definitions_payload_object_check
        CHECK (jsonb_typeof(default_payload) = 'object'),
    CONSTRAINT hrd_job_definitions_version_positive_check CHECK (version > 0)
);

CREATE INDEX hrd_job_definitions_active_idx
    ON hrd_job_definitions (is_active)
    WHERE is_active = true;
CREATE INDEX hrd_job_definitions_job_type_idx
    ON hrd_job_definitions (job_type);

-- Deterministic seeds. Cron defaults are technical choices (BO-editable).
-- TJK: legacy summary cadence Tue/Thu/Sat 00:10 Europe/Istanbul → 6-field with seconds.
-- Expiry: daily 09:00 Europe/Istanbul.
-- Media reconcile: daily 03:30, default inactive (safe).
INSERT INTO hrd_job_definitions (
    id, job_key, name, description, job_type, cron_expression,
    is_active, timeout_seconds, default_payload, supports_reference_date,
    version, created_at, updated_at
) VALUES
    (
        'c0000000-0000-4000-8000-000000000001',
        'TJK_SYNC',
        'TJK horse summary sync',
        'Legacy-aligned TJK bulk summary sync (PageNumber 0-based).',
        'TJK_SYNC',
        '0 10 0 * * 2,4,6',
        false,
        3600,
        '{}'::jsonb,
        false,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c0000000-0000-4000-8000-000000000002',
        'PACKAGE_EXPIRY_SCAN',
        'Package expiry reminder scan',
        'Daily scan for 5-day and 1-day package expiry reminders and expiry.',
        'PACKAGE_EXPIRY_SCAN',
        '0 0 9 * * *',
        true,
        1800,
        '{}'::jsonb,
        true,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c0000000-0000-4000-8000-000000000003',
        'MEDIA_RECONCILE',
        'Media storage reconcile',
        'Reconcile orphaned or stuck media objects against private B2 storage.',
        'MEDIA_RECONCILE',
        '0 30 3 * * *',
        false,
        3600,
        '{}'::jsonb,
        false,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    )
ON CONFLICT (job_key) DO NOTHING;

-- Optional execution metadata on background jobs for BO history (additive, nullable).
ALTER TABLE hrd_background_jobs
    ADD COLUMN execution_type varchar(16) NULL;
ALTER TABLE hrd_background_jobs
    ADD COLUMN triggered_by_user_id uuid NULL;
ALTER TABLE hrd_background_jobs
    ADD COLUMN reference_date date NULL;
ALTER TABLE hrd_background_jobs
    ADD COLUMN job_definition_id uuid NULL;
ALTER TABLE hrd_background_jobs
    ADD COLUMN started_at timestamptz NULL;

ALTER TABLE hrd_background_jobs
    ADD CONSTRAINT hrd_background_jobs_execution_type_check CHECK (
        execution_type IS NULL OR execution_type IN ('SCHEDULED', 'MANUAL')
    );
ALTER TABLE hrd_background_jobs
    ADD CONSTRAINT hrd_background_jobs_triggered_by_user_id_fkey
        FOREIGN KEY (triggered_by_user_id) REFERENCES hrd_users (id) ON DELETE RESTRICT;
ALTER TABLE hrd_background_jobs
    ADD CONSTRAINT hrd_background_jobs_job_definition_id_fkey
        FOREIGN KEY (job_definition_id) REFERENCES hrd_job_definitions (id) ON DELETE RESTRICT;

CREATE INDEX hrd_background_jobs_definition_created_idx
    ON hrd_background_jobs (job_definition_id, created_at DESC)
    WHERE job_definition_id IS NOT NULL;

-- Scheduled occurrence dedup: one QUEUED/LEASED/SUCCEEDED per definition+occurrence.
CREATE UNIQUE INDEX hrd_background_jobs_scheduled_occurrence_key
    ON hrd_background_jobs (job_definition_id, deduplication_key)
    WHERE job_definition_id IS NOT NULL
      AND deduplication_key IS NOT NULL
      AND status IN ('QUEUED', 'LEASED', 'SUCCEEDED');

-- +goose Down
DROP INDEX hrd_background_jobs_scheduled_occurrence_key;
DROP INDEX hrd_background_jobs_definition_created_idx;

ALTER TABLE hrd_background_jobs
    DROP CONSTRAINT hrd_background_jobs_job_definition_id_fkey;
ALTER TABLE hrd_background_jobs
    DROP CONSTRAINT hrd_background_jobs_triggered_by_user_id_fkey;
ALTER TABLE hrd_background_jobs
    DROP CONSTRAINT hrd_background_jobs_execution_type_check;

ALTER TABLE hrd_background_jobs DROP COLUMN started_at;
ALTER TABLE hrd_background_jobs DROP COLUMN job_definition_id;
ALTER TABLE hrd_background_jobs DROP COLUMN reference_date;
ALTER TABLE hrd_background_jobs DROP COLUMN triggered_by_user_id;
ALTER TABLE hrd_background_jobs DROP COLUMN execution_type;

DROP TABLE hrd_job_definitions;
