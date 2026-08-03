-- +goose Up
CREATE TABLE hrd_tjk_sync_runs (
    id uuid NOT NULL,
    mode varchar(32) NOT NULL,
    status varchar(32) NOT NULL DEFAULT 'QUEUED',
    source_adapter varchar(64) NOT NULL,
    scope_key varchar(64) NOT NULL DEFAULT 'HORSES',
    checkpoint jsonb NOT NULL DEFAULT '{}',
    source_snapshot varchar(255) NULL,
    trigger_kind varchar(32) NOT NULL,
    created_by_user_id uuid NULL,
    cancel_requested_at timestamptz NULL,
    cancelled_at timestamptz NULL,
    started_at timestamptz NULL,
    completed_at timestamptz NULL,
    total_count integer NOT NULL DEFAULT 0,
    created_count integer NOT NULL DEFAULT 0,
    updated_count integer NOT NULL DEFAULT 0,
    unchanged_count integer NOT NULL DEFAULT 0,
    skipped_count integer NOT NULL DEFAULT 0,
    failed_count integer NOT NULL DEFAULT 0,
    conflict_count integer NOT NULL DEFAULT 0,
    last_error_summary text NULL,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_tjk_sync_runs_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_tjk_sync_runs_created_by_user_id_fkey FOREIGN KEY (created_by_user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_tjk_sync_runs_mode_check
        CHECK (mode IN ('FULL', 'INCREMENTAL', 'RECONCILIATION')),
    CONSTRAINT hrd_tjk_sync_runs_status_check CHECK (status IN (
        'QUEUED', 'RUNNING', 'SUCCEEDED', 'PARTIAL_SUCCESS', 'FAILED', 'CANCELLED'
    )),
    CONSTRAINT hrd_tjk_sync_runs_trigger_kind_check
        CHECK (trigger_kind IN ('SCHEDULED', 'MANUAL')),
    CONSTRAINT hrd_tjk_sync_runs_scope_key_check CHECK (scope_key = 'HORSES'),
    CONSTRAINT hrd_tjk_sync_runs_checkpoint_object_check
        CHECK (jsonb_typeof(checkpoint) = 'object'),
    CONSTRAINT hrd_tjk_sync_runs_running_started_check
        CHECK (status <> 'RUNNING' OR started_at IS NOT NULL),
    CONSTRAINT hrd_tjk_sync_runs_terminal_completed_check CHECK (
        status NOT IN ('SUCCEEDED', 'PARTIAL_SUCCESS', 'FAILED', 'CANCELLED')
        OR completed_at IS NOT NULL
    ),
    CONSTRAINT hrd_tjk_sync_runs_cancelled_fields_check CHECK (
        status <> 'CANCELLED'
        OR (cancelled_at IS NOT NULL AND cancel_requested_at IS NOT NULL)
    ),
    CONSTRAINT hrd_tjk_sync_runs_trigger_actor_check CHECK (
        (trigger_kind = 'MANUAL' AND created_by_user_id IS NOT NULL)
        OR (trigger_kind = 'SCHEDULED' AND created_by_user_id IS NULL)
    ),
    CONSTRAINT hrd_tjk_sync_runs_time_order_check CHECK (
        (started_at IS NULL OR started_at >= created_at)
        AND (completed_at IS NULL OR (started_at IS NOT NULL AND completed_at >= started_at))
        AND (cancelled_at IS NULL OR cancelled_at >= created_at)
    ),
    CONSTRAINT hrd_tjk_sync_runs_version_positive_check CHECK (version > 0),
    CONSTRAINT hrd_tjk_sync_runs_total_count_nonnegative_check CHECK (total_count >= 0),
    CONSTRAINT hrd_tjk_sync_runs_created_count_nonnegative_check CHECK (created_count >= 0),
    CONSTRAINT hrd_tjk_sync_runs_updated_count_nonnegative_check CHECK (updated_count >= 0),
    CONSTRAINT hrd_tjk_sync_runs_unchanged_count_nonnegative_check CHECK (unchanged_count >= 0),
    CONSTRAINT hrd_tjk_sync_runs_skipped_count_nonnegative_check CHECK (skipped_count >= 0),
    CONSTRAINT hrd_tjk_sync_runs_failed_count_nonnegative_check CHECK (failed_count >= 0),
    CONSTRAINT hrd_tjk_sync_runs_conflict_count_nonnegative_check CHECK (conflict_count >= 0)
);

CREATE UNIQUE INDEX hrd_tjk_sync_runs_one_active_per_source_scope_key
    ON hrd_tjk_sync_runs (source_adapter, scope_key)
    WHERE status IN ('QUEUED', 'RUNNING');
CREATE INDEX hrd_tjk_sync_runs_created_idx ON hrd_tjk_sync_runs (created_at DESC);
CREATE INDEX hrd_tjk_sync_runs_status_idx ON hrd_tjk_sync_runs (status);
CREATE INDEX hrd_tjk_sync_runs_source_scope_created_idx
    ON hrd_tjk_sync_runs (source_adapter, scope_key, created_at DESC);

CREATE TABLE hrd_background_jobs (
    id uuid NOT NULL,
    job_type varchar(64) NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'QUEUED',
    payload jsonb NOT NULL DEFAULT '{}',
    tjk_sync_run_id uuid NULL,
    deduplication_key varchar(255) NULL,
    attempt_count integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL,
    available_at timestamptz NOT NULL,
    leased_until timestamptz NULL,
    lease_owner varchar(128) NULL,
    last_error text NULL,
    cancel_requested_at timestamptz NULL,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    completed_at timestamptz NULL,
    CONSTRAINT hrd_background_jobs_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_background_jobs_tjk_sync_run_id_fkey FOREIGN KEY (tjk_sync_run_id)
        REFERENCES hrd_tjk_sync_runs (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_background_jobs_job_type_check CHECK (job_type IN (
        'TJK_SYNC_BATCH', 'MEDIA_VALIDATE_AND_NORMALIZE', 'MEDIA_GENERATE_VARIANT',
        'MEDIA_DELETE_OBJECTS', 'MEDIA_RECONCILE'
    )),
    CONSTRAINT hrd_background_jobs_status_check CHECK (status IN (
        'QUEUED', 'LEASED', 'SUCCEEDED', 'FAILED', 'CANCELLED', 'DEAD'
    )),
    CONSTRAINT hrd_background_jobs_payload_object_check
        CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT hrd_background_jobs_attempt_bounds_check
        CHECK (attempt_count >= 0 AND attempt_count <= max_attempts),
    CONSTRAINT hrd_background_jobs_max_attempts_positive_check CHECK (max_attempts > 0),
    CONSTRAINT hrd_background_jobs_tjk_run_by_job_type_check CHECK (
        (job_type = 'TJK_SYNC_BATCH' AND tjk_sync_run_id IS NOT NULL)
        OR (job_type <> 'TJK_SYNC_BATCH' AND tjk_sync_run_id IS NULL)
    ),
    CONSTRAINT hrd_background_jobs_lease_fields_check CHECK (
        (status = 'LEASED' AND lease_owner IS NOT NULL AND leased_until IS NOT NULL)
        OR (status <> 'LEASED' AND lease_owner IS NULL AND leased_until IS NULL)
    ),
    CONSTRAINT hrd_background_jobs_completed_at_terminal_check CHECK (
        (status IN ('SUCCEEDED', 'FAILED', 'DEAD', 'CANCELLED') AND completed_at IS NOT NULL)
        OR (status IN ('QUEUED', 'LEASED') AND completed_at IS NULL)
    ),
    CONSTRAINT hrd_background_jobs_cancelled_requires_request_check
        CHECK (status <> 'CANCELLED' OR cancel_requested_at IS NOT NULL),
    CONSTRAINT hrd_background_jobs_version_positive_check CHECK (version > 0)
);

CREATE UNIQUE INDEX hrd_background_jobs_deduplication_key_key
    ON hrd_background_jobs (deduplication_key)
    WHERE deduplication_key IS NOT NULL;
CREATE INDEX hrd_background_jobs_claim_idx
    ON hrd_background_jobs (status, available_at, id)
    WHERE status = 'QUEUED';
CREATE INDEX hrd_background_jobs_lease_recovery_idx
    ON hrd_background_jobs (status, leased_until)
    WHERE status = 'LEASED';
CREATE INDEX hrd_background_jobs_tjk_sync_run_id_idx
    ON hrd_background_jobs (tjk_sync_run_id);

CREATE TABLE hrd_tjk_sync_item_errors (
    id uuid NOT NULL,
    run_id uuid NOT NULL,
    tjk_number varchar(64) NULL,
    horse_id uuid NULL,
    batch_context jsonb NOT NULL DEFAULT '{}',
    error_class varchar(16) NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'OPEN',
    message text NOT NULL,
    detail jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL,
    resolved_at timestamptz NULL,
    CONSTRAINT hrd_tjk_sync_item_errors_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_tjk_sync_item_errors_run_id_fkey FOREIGN KEY (run_id)
        REFERENCES hrd_tjk_sync_runs (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_tjk_sync_item_errors_horse_id_fkey FOREIGN KEY (horse_id)
        REFERENCES hrd_horses (id) ON DELETE SET NULL,
    CONSTRAINT hrd_tjk_sync_item_errors_error_class_check
        CHECK (error_class IN ('TRANSIENT', 'PERMANENT', 'CONFLICT')),
    CONSTRAINT hrd_tjk_sync_item_errors_status_check
        CHECK (status IN ('OPEN', 'RESOLVED', 'IGNORED')),
    CONSTRAINT hrd_tjk_sync_item_errors_resolution_check CHECK (
        (status = 'OPEN' AND resolved_at IS NULL)
        OR (status IN ('RESOLVED', 'IGNORED') AND resolved_at IS NOT NULL)
    ),
    CONSTRAINT hrd_tjk_sync_item_errors_message_not_blank_check
        CHECK (btrim(message) <> ''),
    CONSTRAINT hrd_tjk_sync_item_errors_batch_context_object_check
        CHECK (jsonb_typeof(batch_context) = 'object'),
    CONSTRAINT hrd_tjk_sync_item_errors_detail_object_check
        CHECK (jsonb_typeof(detail) = 'object')
);

CREATE INDEX hrd_tjk_sync_item_errors_run_status_idx
    ON hrd_tjk_sync_item_errors (run_id, status);
CREATE INDEX hrd_tjk_sync_item_errors_tjk_number_idx
    ON hrd_tjk_sync_item_errors (tjk_number);

-- +goose Down
DROP TABLE hrd_tjk_sync_item_errors;
DROP TABLE hrd_background_jobs;
DROP TABLE hrd_tjk_sync_runs;
