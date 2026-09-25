-- +goose Up
-- Add is_compressed flag to hrd_media_assets and hrd_media_variants
ALTER TABLE hrd_media_assets
    ADD COLUMN is_compressed boolean NOT NULL DEFAULT false;

ALTER TABLE hrd_media_variants
    ADD COLUMN is_compressed boolean NOT NULL DEFAULT false;

CREATE INDEX hrd_media_assets_uncompressed_idx
    ON hrd_media_assets (is_compressed)
    WHERE is_compressed = false;

CREATE INDEX hrd_media_variants_uncompressed_idx
    ON hrd_media_variants (is_compressed)
    WHERE is_compressed = false;

-- Allow MEDIA_BATCH_COMPRESS in hrd_background_jobs
ALTER TABLE hrd_background_jobs
    DROP CONSTRAINT hrd_background_jobs_job_type_check;

ALTER TABLE hrd_background_jobs
    ADD CONSTRAINT hrd_background_jobs_job_type_check CHECK (job_type IN (
        'TJK_SYNC_BATCH',
        'MEDIA_VALIDATE_AND_NORMALIZE',
        'MEDIA_GENERATE_VARIANT',
        'MEDIA_DELETE_OBJECTS',
        'MEDIA_RECONCILE',
        'MEDIA_BATCH_COMPRESS',
        'NOTIFICATION_FANOUT_ADVANCED_ADVERT',
        'NOTIFICATION_FANOUT_PACKAGE_ADVERT',
        'NOTIFICATION_FANOUT_URGENT_ADVERT',
        'NOTIFICATION_FANOUT_ADVERT_PRICE_DROP',
        'EMAIL_SEND_ADVERT_NOTIFICATION_CHUNK',
        'PACKAGE_EXPIRY_REMINDER_SCAN',
        'EMAIL_SEND_PACKAGE_EXPIRY_REMINDER'
    ));

-- Allow MEDIA_BATCH_COMPRESS in hrd_job_definitions
ALTER TABLE hrd_job_definitions
    DROP CONSTRAINT hrd_job_definitions_job_type_check;

ALTER TABLE hrd_job_definitions
    ADD CONSTRAINT hrd_job_definitions_job_type_check CHECK (job_type IN (
        'TJK_SYNC',
        'PACKAGE_EXPIRY_SCAN',
        'MEDIA_RECONCILE',
        'MEDIA_BATCH_COMPRESS'
    ));

-- Seed job definition for MEDIA_BATCH_COMPRESS
INSERT INTO hrd_job_definitions (
    id, job_key, name, description, job_type, cron_expression,
    is_active, timeout_seconds, default_payload, supports_reference_date,
    version, created_at, updated_at
) VALUES (
    'c0000000-0000-4000-8000-000000000004',
    'MEDIA_BATCH_COMPRESS',
    'Sıkıştırılmamış Görselleri Toplu Sıkıştırma (TinyPNG)',
    'Kota veya kesinti nedeniyle sıkıştırılamayan görselleri TinyPNG ile toplu olarak sıkıştırır.',
    'MEDIA_BATCH_COMPRESS',
    '0 0 0 1 * *',
    true,
    3600,
    '{}'::jsonb,
    false,
    1,
    NOW(),
    NOW()
) ON CONFLICT (job_key) DO NOTHING;

-- +goose Down
DELETE FROM hrd_job_definitions WHERE job_key = 'MEDIA_BATCH_COMPRESS';

ALTER TABLE hrd_job_definitions
    DROP CONSTRAINT hrd_job_definitions_job_type_check;

ALTER TABLE hrd_job_definitions
    ADD CONSTRAINT hrd_job_definitions_job_type_check CHECK (job_type IN (
        'TJK_SYNC',
        'PACKAGE_EXPIRY_SCAN',
        'MEDIA_RECONCILE'
    ));

ALTER TABLE hrd_background_jobs
    DROP CONSTRAINT hrd_background_jobs_job_type_check;

ALTER TABLE hrd_background_jobs
    ADD CONSTRAINT hrd_background_jobs_job_type_check CHECK (job_type IN (
        'TJK_SYNC_BATCH',
        'MEDIA_VALIDATE_AND_NORMALIZE',
        'MEDIA_GENERATE_VARIANT',
        'MEDIA_DELETE_OBJECTS',
        'MEDIA_RECONCILE',
        'NOTIFICATION_FANOUT_ADVANCED_ADVERT',
        'NOTIFICATION_FANOUT_PACKAGE_ADVERT',
        'NOTIFICATION_FANOUT_URGENT_ADVERT',
        'NOTIFICATION_FANOUT_ADVERT_PRICE_DROP',
        'EMAIL_SEND_ADVERT_NOTIFICATION_CHUNK',
        'PACKAGE_EXPIRY_REMINDER_SCAN',
        'EMAIL_SEND_PACKAGE_EXPIRY_REMINDER'
    ));

DROP INDEX IF EXISTS hrd_media_variants_uncompressed_idx;
DROP INDEX IF EXISTS hrd_media_assets_uncompressed_idx;
ALTER TABLE hrd_media_variants DROP COLUMN IF EXISTS is_compressed;
ALTER TABLE hrd_media_assets DROP COLUMN IF EXISTS is_compressed;
