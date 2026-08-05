-- +goose Up
-- Dynamic package catalog, broadcast capability, generic publish event, 5/1 reminders.

ALTER TABLE hrd_packages
    DROP CONSTRAINT hrd_packages_code_check;
ALTER TABLE hrd_packages
    ADD CONSTRAINT hrd_packages_code_format_check
        CHECK (code ~ '^[A-Z0-9][A-Z0-9_-]{1,63}$');

ALTER TABLE hrd_packages
    ADD COLUMN broadcast_on_publish boolean NOT NULL DEFAULT false;

-- Preserve upgrade history: only remove untouched deterministic seeds that are
-- unreferenced. Referenced or edited rows stay (silent history loss forbidden).
DELETE FROM hrd_packages p
WHERE p.id IN (
        'a0000000-0000-4000-8000-000000000001'::uuid,
        'a0000000-0000-4000-8000-000000000002'::uuid,
        'a0000000-0000-4000-8000-000000000003'::uuid
    )
    AND p.version = 1
    AND p.created_at = TIMESTAMPTZ '2020-01-01 00:00:00+00'
    AND p.updated_at = TIMESTAMPTZ '2020-01-01 00:00:00+00'
    AND p.description IS NULL
    AND p.badge_text IS NULL
    AND p.benefits = '[]'::jsonb
    AND p.display_price_amount_minor IS NULL
    AND p.currency_code = 'TRY'
    AND p.default_duration_days IS NULL
    AND p.broadcast_on_publish = false
    AND NOT EXISTS (
        SELECT 1 FROM hrd_advert_package_assignments a WHERE a.package_id = p.id
    )
    AND NOT EXISTS (
        SELECT 1 FROM hrd_campaigns c
        WHERE c.source_package_id = p.id OR c.target_package_id = p.id
    );

-- Drop event CHECKs before remapping values.
ALTER TABLE hrd_campaigns
    DROP CONSTRAINT hrd_campaigns_event_type_check;
ALTER TABLE hrd_notification_templates
    DROP CONSTRAINT hrd_notification_templates_event_type_check;
ALTER TABLE hrd_notifications
    DROP CONSTRAINT hrd_notifications_event_type_check;

-- Campaign event types: map 10/3 → 5/1, then tighten CHECK.
UPDATE hrd_campaigns
SET event_type = 'PACKAGE_EXPIRY_5_DAYS'
WHERE event_type = 'PACKAGE_EXPIRY_10_DAYS';
UPDATE hrd_campaigns
SET event_type = 'PACKAGE_EXPIRY_1_DAY'
WHERE event_type = 'PACKAGE_EXPIRY_3_DAYS';

ALTER TABLE hrd_campaigns
    ADD CONSTRAINT hrd_campaigns_event_type_check CHECK (event_type IN (
        'PACKAGE_EXPIRY_5_DAYS',
        'PACKAGE_EXPIRY_1_DAY',
        'PACKAGE_RENEWAL',
        'PACKAGE_UPGRADE'
    ));

-- Notification templates: rename events in place (unique on event_type).
UPDATE hrd_notification_templates
SET event_type = 'PACKAGE_ADVERT_PUBLISHED',
    name = 'Package advert published placeholder',
    updated_at = TIMESTAMPTZ '2020-01-01 00:00:00+00'
WHERE event_type = 'ADVANCED_ADVERT_PUBLISHED';

UPDATE hrd_notification_templates
SET event_type = 'PACKAGE_EXPIRY_5_DAYS',
    name = 'Package expiry 5 days placeholder',
    in_app_title_template = 'Paket süresi',
    in_app_body_template = 'Paket süreniz yakında dolacak.',
    updated_at = TIMESTAMPTZ '2020-01-01 00:00:00+00'
WHERE event_type = 'PACKAGE_EXPIRY_10_DAYS';

UPDATE hrd_notification_templates
SET event_type = 'PACKAGE_EXPIRY_1_DAY',
    name = 'Package expiry 1 day placeholder',
    in_app_title_template = 'Paket süresi',
    in_app_body_template = 'Paket süreniz yakında dolacak.',
    updated_at = TIMESTAMPTZ '2020-01-01 00:00:00+00'
WHERE event_type = 'PACKAGE_EXPIRY_3_DAYS';

ALTER TABLE hrd_notification_templates
    ADD CONSTRAINT hrd_notification_templates_event_type_check CHECK (event_type IN (
        'PACKAGE_ADVERT_PUBLISHED',
        'URGENT_ADVERT_ACTIVATED',
        'PACKAGE_EXPIRY_5_DAYS',
        'PACKAGE_EXPIRY_1_DAY'
    ));

-- Historical notification rows: map event_type; keep event_key for identity.
UPDATE hrd_notifications
SET event_type = 'PACKAGE_ADVERT_PUBLISHED'
WHERE event_type = 'ADVANCED_ADVERT_PUBLISHED';
UPDATE hrd_notifications
SET event_type = 'PACKAGE_EXPIRY_5_DAYS'
WHERE event_type = 'PACKAGE_EXPIRY_10_DAYS';
UPDATE hrd_notifications
SET event_type = 'PACKAGE_EXPIRY_1_DAY'
WHERE event_type = 'PACKAGE_EXPIRY_3_DAYS';

ALTER TABLE hrd_notifications
    ADD CONSTRAINT hrd_notifications_event_type_check CHECK (event_type IN (
        'PACKAGE_ADVERT_PUBLISHED',
        'URGENT_ADVERT_ACTIVATED',
        'PACKAGE_EXPIRY_5_DAYS',
        'PACKAGE_EXPIRY_1_DAY',
        'ADVANCED_ADVERT_PUBLISHED',
        'PACKAGE_EXPIRY_10_DAYS',
        'PACKAGE_EXPIRY_3_DAYS'
    ));

-- Job types: add generic fan-out; keep ADVANCED fan-out for historical rows.
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
        'EMAIL_SEND_ADVERT_NOTIFICATION_CHUNK',
        'PACKAGE_EXPIRY_REMINDER_SCAN',
        'EMAIL_SEND_PACKAGE_EXPIRY_REMINDER'
    ));

-- +goose Down
-- Down fails safely if rows use new event/job values or broadcast-only packages.

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
        'NOTIFICATION_FANOUT_URGENT_ADVERT',
        'EMAIL_SEND_ADVERT_NOTIFICATION_CHUNK',
        'PACKAGE_EXPIRY_REMINDER_SCAN',
        'EMAIL_SEND_PACKAGE_EXPIRY_REMINDER'
    ));

ALTER TABLE hrd_notifications
    DROP CONSTRAINT hrd_notifications_event_type_check;
ALTER TABLE hrd_notification_templates
    DROP CONSTRAINT hrd_notification_templates_event_type_check;
ALTER TABLE hrd_campaigns
    DROP CONSTRAINT hrd_campaigns_event_type_check;

UPDATE hrd_notifications
SET event_type = 'ADVANCED_ADVERT_PUBLISHED'
WHERE event_type = 'PACKAGE_ADVERT_PUBLISHED';
UPDATE hrd_notifications
SET event_type = 'PACKAGE_EXPIRY_10_DAYS'
WHERE event_type = 'PACKAGE_EXPIRY_5_DAYS';
UPDATE hrd_notifications
SET event_type = 'PACKAGE_EXPIRY_3_DAYS'
WHERE event_type = 'PACKAGE_EXPIRY_1_DAY';

ALTER TABLE hrd_notifications
    ADD CONSTRAINT hrd_notifications_event_type_check CHECK (event_type IN (
        'ADVANCED_ADVERT_PUBLISHED',
        'URGENT_ADVERT_ACTIVATED',
        'PACKAGE_EXPIRY_10_DAYS',
        'PACKAGE_EXPIRY_3_DAYS'
    ));

UPDATE hrd_notification_templates
SET event_type = 'ADVANCED_ADVERT_PUBLISHED',
    name = 'Advanced advert published placeholder',
    updated_at = TIMESTAMPTZ '2020-01-01 00:00:00+00'
WHERE event_type = 'PACKAGE_ADVERT_PUBLISHED';
UPDATE hrd_notification_templates
SET event_type = 'PACKAGE_EXPIRY_10_DAYS',
    name = 'Package expiry 10 days placeholder',
    updated_at = TIMESTAMPTZ '2020-01-01 00:00:00+00'
WHERE event_type = 'PACKAGE_EXPIRY_5_DAYS';
UPDATE hrd_notification_templates
SET event_type = 'PACKAGE_EXPIRY_3_DAYS',
    name = 'Package expiry 3 days placeholder',
    updated_at = TIMESTAMPTZ '2020-01-01 00:00:00+00'
WHERE event_type = 'PACKAGE_EXPIRY_1_DAY';

ALTER TABLE hrd_notification_templates
    ADD CONSTRAINT hrd_notification_templates_event_type_check CHECK (event_type IN (
        'ADVANCED_ADVERT_PUBLISHED',
        'URGENT_ADVERT_ACTIVATED',
        'PACKAGE_EXPIRY_10_DAYS',
        'PACKAGE_EXPIRY_3_DAYS'
    ));

UPDATE hrd_campaigns
SET event_type = 'PACKAGE_EXPIRY_10_DAYS'
WHERE event_type = 'PACKAGE_EXPIRY_5_DAYS';
UPDATE hrd_campaigns
SET event_type = 'PACKAGE_EXPIRY_3_DAYS'
WHERE event_type = 'PACKAGE_EXPIRY_1_DAY';

ALTER TABLE hrd_campaigns
    ADD CONSTRAINT hrd_campaigns_event_type_check CHECK (event_type IN (
        'PACKAGE_EXPIRY_10_DAYS',
        'PACKAGE_EXPIRY_3_DAYS',
        'PACKAGE_RENEWAL',
        'PACKAGE_UPGRADE'
    ));

ALTER TABLE hrd_packages
    DROP COLUMN broadcast_on_publish;

ALTER TABLE hrd_packages
    DROP CONSTRAINT hrd_packages_code_format_check;
ALTER TABLE hrd_packages
    ADD CONSTRAINT hrd_packages_code_check CHECK (code IN ('STARTER', 'MIDDLE', 'ADVANCED'));
