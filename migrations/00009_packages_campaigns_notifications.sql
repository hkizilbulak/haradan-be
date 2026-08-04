-- +goose Up
CREATE TABLE hrd_packages (
    id uuid NOT NULL,
    code varchar(32) NOT NULL,
    display_name varchar(120) NOT NULL,
    description text NULL,
    badge_text varchar(64) NULL,
    benefits jsonb NOT NULL DEFAULT '[]'::jsonb,
    display_price_amount_minor bigint NULL,
    currency_code varchar(3) NOT NULL DEFAULT 'TRY',
    default_duration_days integer NULL,
    allows_urgent boolean NOT NULL DEFAULT false,
    showcase_eligible boolean NOT NULL DEFAULT false,
    search_priority integer NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true,
    sort_order integer NOT NULL DEFAULT 0,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_packages_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_packages_code_key UNIQUE (code),
    CONSTRAINT hrd_packages_code_check CHECK (code IN ('STARTER', 'MIDDLE', 'ADVANCED')),
    CONSTRAINT hrd_packages_display_name_not_blank_check
        CHECK (btrim(display_name) <> ''),
    CONSTRAINT hrd_packages_benefits_array_check
        CHECK (jsonb_typeof(benefits) = 'array'),
    CONSTRAINT hrd_packages_display_price_nonnegative_check
        CHECK (display_price_amount_minor IS NULL OR display_price_amount_minor >= 0),
    CONSTRAINT hrd_packages_currency_code_format_check
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT hrd_packages_default_duration_positive_check
        CHECK (default_duration_days IS NULL OR default_duration_days > 0),
    CONSTRAINT hrd_packages_search_priority_nonnegative_check
        CHECK (search_priority >= 0),
    CONSTRAINT hrd_packages_sort_order_nonnegative_check
        CHECK (sort_order >= 0),
    CONSTRAINT hrd_packages_version_positive_check CHECK (version > 0)
);

CREATE INDEX hrd_packages_active_sort_idx
    ON hrd_packages (sort_order, code)
    WHERE is_active = true;

INSERT INTO hrd_packages (
    id, code, display_name, description, badge_text, benefits,
    display_price_amount_minor, currency_code, default_duration_days,
    allows_urgent, showcase_eligible, search_priority, is_active, sort_order,
    version, created_at, updated_at
) VALUES
    (
        'a0000000-0000-4000-8000-000000000001',
        'STARTER',
        'Starter',
        NULL,
        NULL,
        '[]'::jsonb,
        NULL,
        'TRY',
        NULL,
        false,
        false,
        0,
        true,
        10,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'a0000000-0000-4000-8000-000000000002',
        'MIDDLE',
        'Middle',
        NULL,
        NULL,
        '[]'::jsonb,
        NULL,
        'TRY',
        NULL,
        false,
        true,
        0,
        true,
        20,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'a0000000-0000-4000-8000-000000000003',
        'ADVANCED',
        'Advanced',
        NULL,
        NULL,
        '[]'::jsonb,
        NULL,
        'TRY',
        NULL,
        true,
        true,
        100,
        true,
        30,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    );

CREATE TABLE hrd_advert_package_assignments (
    id uuid NOT NULL,
    advert_id uuid NOT NULL,
    package_id uuid NOT NULL,
    status varchar(16) NOT NULL,
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NULL,
    assigned_by_user_id uuid NOT NULL,
    assigned_at timestamptz NOT NULL,
    superseded_at timestamptz NULL,
    expired_at timestamptz NULL,
    cancelled_at timestamptz NULL,
    reason text NULL,
    source varchar(16) NOT NULL,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_advert_package_assignments_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_advert_package_assignments_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_advert_package_assignments_package_id_fkey FOREIGN KEY (package_id)
        REFERENCES hrd_packages (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_advert_package_assignments_assigned_by_user_id_fkey FOREIGN KEY (assigned_by_user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_advert_package_assignments_status_check CHECK (status IN (
        'ACTIVE', 'SUPERSEDED', 'EXPIRED', 'CANCELLED'
    )),
    CONSTRAINT hrd_advert_package_assignments_source_check CHECK (source IN (
        'ADMIN', 'SYSTEM'
    )),
    CONSTRAINT hrd_advert_package_assignments_starts_ends_check
        CHECK (ends_at IS NULL OR starts_at <= ends_at),
    CONSTRAINT hrd_advert_package_assignments_status_timestamps_check CHECK (
        (
            status = 'ACTIVE'
            AND superseded_at IS NULL
            AND expired_at IS NULL
            AND cancelled_at IS NULL
        )
        OR (
            status = 'SUPERSEDED'
            AND superseded_at IS NOT NULL
            AND expired_at IS NULL
            AND cancelled_at IS NULL
        )
        OR (
            status = 'EXPIRED'
            AND expired_at IS NOT NULL
            AND superseded_at IS NULL
            AND cancelled_at IS NULL
        )
        OR (
            status = 'CANCELLED'
            AND cancelled_at IS NOT NULL
            AND superseded_at IS NULL
            AND expired_at IS NULL
        )
    ),
    CONSTRAINT hrd_advert_package_assignments_version_positive_check CHECK (version > 0)
);

CREATE UNIQUE INDEX hrd_advert_package_assignments_one_active_per_advert_key
    ON hrd_advert_package_assignments (advert_id)
    WHERE status = 'ACTIVE';
CREATE INDEX hrd_advert_package_assignments_advert_assigned_idx
    ON hrd_advert_package_assignments (advert_id, assigned_at DESC, id DESC);
CREATE INDEX hrd_advert_package_assignments_package_status_idx
    ON hrd_advert_package_assignments (package_id, status);
CREATE INDEX hrd_advert_package_assignments_active_package_idx
    ON hrd_advert_package_assignments (package_id, advert_id)
    WHERE status = 'ACTIVE';
CREATE INDEX hrd_advert_package_assignments_active_ends_at_idx
    ON hrd_advert_package_assignments (ends_at)
    WHERE status = 'ACTIVE' AND ends_at IS NOT NULL;

CREATE TABLE hrd_advert_feature_activations (
    id uuid NOT NULL,
    advert_id uuid NOT NULL,
    package_assignment_id uuid NOT NULL,
    feature_code varchar(32) NOT NULL,
    status varchar(16) NOT NULL,
    activated_by_user_id uuid NOT NULL,
    activated_at timestamptz NOT NULL,
    deactivated_at timestamptz NULL,
    reason text NULL,
    activation_version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_advert_feature_activations_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_advert_feature_activations_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_advert_feature_activations_package_assignment_id_fkey FOREIGN KEY (package_assignment_id)
        REFERENCES hrd_advert_package_assignments (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_advert_feature_activations_activated_by_user_id_fkey FOREIGN KEY (activated_by_user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_advert_feature_activations_feature_code_check
        CHECK (feature_code IN ('URGENT')),
    CONSTRAINT hrd_advert_feature_activations_status_check CHECK (status IN (
        'ACTIVE', 'DEACTIVATED'
    )),
    CONSTRAINT hrd_advert_feature_activations_status_timestamps_check CHECK (
        (status = 'ACTIVE' AND deactivated_at IS NULL)
        OR (status = 'DEACTIVATED' AND deactivated_at IS NOT NULL)
    ),
    CONSTRAINT hrd_advert_feature_activations_act_version_positive_check
        CHECK (activation_version > 0)
);

CREATE UNIQUE INDEX hrd_advert_feature_activations_one_active_feature_key
    ON hrd_advert_feature_activations (advert_id, feature_code)
    WHERE status = 'ACTIVE';
CREATE INDEX hrd_advert_feature_activations_active_urgent_idx
    ON hrd_advert_feature_activations (advert_id)
    WHERE status = 'ACTIVE' AND feature_code = 'URGENT';
CREATE INDEX hrd_advert_feature_activations_assignment_idx
    ON hrd_advert_feature_activations (package_assignment_id);

CREATE TABLE hrd_campaigns (
    id uuid NOT NULL,
    code varchar(64) NOT NULL,
    name varchar(160) NOT NULL,
    event_type varchar(64) NOT NULL,
    source_package_id uuid NULL,
    target_package_id uuid NULL,
    title varchar(200) NOT NULL,
    description text NULL,
    email_subject varchar(200) NULL,
    email_heading varchar(200) NULL,
    email_body text NULL,
    cta_label varchar(120) NULL,
    cta_url text NULL,
    badge_text varchar(64) NULL,
    image_asset_id uuid NULL,
    display_original_price_amount_minor bigint NULL,
    display_campaign_price_amount_minor bigint NULL,
    currency_code varchar(3) NOT NULL DEFAULT 'TRY',
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_by_user_id uuid NOT NULL,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_campaigns_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_campaigns_code_key UNIQUE (code),
    CONSTRAINT hrd_campaigns_source_package_id_fkey FOREIGN KEY (source_package_id)
        REFERENCES hrd_packages (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_campaigns_target_package_id_fkey FOREIGN KEY (target_package_id)
        REFERENCES hrd_packages (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_campaigns_image_asset_id_fkey FOREIGN KEY (image_asset_id)
        REFERENCES hrd_media_assets (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_campaigns_created_by_user_id_fkey FOREIGN KEY (created_by_user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_campaigns_event_type_check CHECK (event_type IN (
        'PACKAGE_EXPIRY_10_DAYS',
        'PACKAGE_EXPIRY_3_DAYS',
        'PACKAGE_RENEWAL',
        'PACKAGE_UPGRADE'
    )),
    CONSTRAINT hrd_campaigns_name_not_blank_check CHECK (btrim(name) <> ''),
    CONSTRAINT hrd_campaigns_title_not_blank_check CHECK (btrim(title) <> ''),
    CONSTRAINT hrd_campaigns_original_price_nonnegative_check
        CHECK (display_original_price_amount_minor IS NULL OR display_original_price_amount_minor >= 0),
    CONSTRAINT hrd_campaigns_campaign_price_nonnegative_check
        CHECK (display_campaign_price_amount_minor IS NULL OR display_campaign_price_amount_minor >= 0),
    CONSTRAINT hrd_campaigns_campaign_price_lte_original_check CHECK (
        display_original_price_amount_minor IS NULL
        OR display_campaign_price_amount_minor IS NULL
        OR display_campaign_price_amount_minor <= display_original_price_amount_minor
    ),
    CONSTRAINT hrd_campaigns_currency_code_format_check
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT hrd_campaigns_starts_ends_check
        CHECK (ends_at IS NULL OR starts_at <= ends_at),
    CONSTRAINT hrd_campaigns_version_positive_check CHECK (version > 0)
);

CREATE INDEX hrd_campaigns_event_active_idx
    ON hrd_campaigns (event_type, is_active);
CREATE INDEX hrd_campaigns_active_window_idx
    ON hrd_campaigns (starts_at, ends_at)
    WHERE is_active = true;
CREATE INDEX hrd_campaigns_source_package_id_idx
    ON hrd_campaigns (source_package_id);
CREATE INDEX hrd_campaigns_target_package_id_idx
    ON hrd_campaigns (target_package_id);

CREATE TABLE hrd_notification_templates (
    id uuid NOT NULL,
    event_type varchar(64) NOT NULL,
    name varchar(160) NOT NULL,
    in_app_title_template varchar(200) NOT NULL,
    in_app_body_template text NOT NULL,
    resend_template_id varchar(128) NULL,
    email_subject_fallback varchar(200) NULL,
    is_active boolean NOT NULL DEFAULT false,
    version integer NOT NULL DEFAULT 1,
    updated_by_user_id uuid NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_notification_templates_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_notification_templates_event_type_key UNIQUE (event_type),
    CONSTRAINT hrd_notification_templates_updated_by_user_id_fkey FOREIGN KEY (updated_by_user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_notification_templates_event_type_check CHECK (event_type IN (
        'ADVANCED_ADVERT_PUBLISHED',
        'URGENT_ADVERT_ACTIVATED',
        'PACKAGE_EXPIRY_10_DAYS',
        'PACKAGE_EXPIRY_3_DAYS'
    )),
    CONSTRAINT hrd_notification_templates_name_not_blank_check
        CHECK (btrim(name) <> ''),
    CONSTRAINT hrd_notification_templates_title_not_blank_check
        CHECK (btrim(in_app_title_template) <> ''),
    CONSTRAINT hrd_notification_templates_body_not_blank_check
        CHECK (btrim(in_app_body_template) <> ''),
    CONSTRAINT hrd_notification_templates_version_positive_check CHECK (version > 0)
);

INSERT INTO hrd_notification_templates (
    id, event_type, name, in_app_title_template, in_app_body_template,
    resend_template_id, email_subject_fallback, is_active, version,
    updated_by_user_id, created_at, updated_at
) VALUES
    (
        'b0000000-0000-4000-8000-000000000001',
        'ADVANCED_ADVERT_PUBLISHED',
        'Advanced advert published placeholder',
        'Yeni ilan',
        'Yeni bir ilan yayınlandı.',
        NULL,
        NULL,
        false,
        1,
        NULL,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'b0000000-0000-4000-8000-000000000002',
        'URGENT_ADVERT_ACTIVATED',
        'Urgent advert activated placeholder',
        'Acil ilan',
        'Bir ilan acil olarak işaretlendi.',
        NULL,
        NULL,
        false,
        1,
        NULL,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'b0000000-0000-4000-8000-000000000003',
        'PACKAGE_EXPIRY_10_DAYS',
        'Package expiry 10 days placeholder',
        'Paket süresi',
        'Paket süreniz yakında dolacak.',
        NULL,
        NULL,
        false,
        1,
        NULL,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'b0000000-0000-4000-8000-000000000004',
        'PACKAGE_EXPIRY_3_DAYS',
        'Package expiry 3 days placeholder',
        'Paket süresi',
        'Paket süreniz yakında dolacak.',
        NULL,
        NULL,
        false,
        1,
        NULL,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    );

CREATE TABLE hrd_notifications (
    id uuid NOT NULL,
    event_type varchar(64) NOT NULL,
    event_key varchar(255) NOT NULL,
    advert_id uuid NULL,
    package_assignment_id uuid NULL,
    campaign_id uuid NULL,
    template_id uuid NULL,
    title varchar(200) NOT NULL,
    body text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL,
    CONSTRAINT hrd_notifications_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_notifications_event_key_key UNIQUE (event_key),
    CONSTRAINT hrd_notifications_event_key_not_blank_check
        CHECK (btrim(event_key) <> ''),
    CONSTRAINT hrd_notifications_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_notifications_package_assignment_id_fkey FOREIGN KEY (package_assignment_id)
        REFERENCES hrd_advert_package_assignments (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_notifications_campaign_id_fkey FOREIGN KEY (campaign_id)
        REFERENCES hrd_campaigns (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_notifications_template_id_fkey FOREIGN KEY (template_id)
        REFERENCES hrd_notification_templates (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_notifications_event_type_check CHECK (event_type IN (
        'ADVANCED_ADVERT_PUBLISHED',
        'URGENT_ADVERT_ACTIVATED',
        'PACKAGE_EXPIRY_10_DAYS',
        'PACKAGE_EXPIRY_3_DAYS'
    )),
    CONSTRAINT hrd_notifications_title_not_blank_check CHECK (btrim(title) <> ''),
    CONSTRAINT hrd_notifications_body_not_blank_check CHECK (btrim(body) <> ''),
    CONSTRAINT hrd_notifications_payload_object_check
        CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX hrd_notifications_created_id_idx
    ON hrd_notifications (created_at DESC, id DESC);
CREATE INDEX hrd_notifications_event_type_created_idx
    ON hrd_notifications (event_type, created_at DESC);

CREATE TABLE hrd_user_notification_states (
    user_id uuid NOT NULL,
    notification_id uuid NOT NULL,
    delivered_at timestamptz NOT NULL,
    read_at timestamptz NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_user_notification_states_pkey PRIMARY KEY (user_id, notification_id),
    CONSTRAINT hrd_user_notification_states_user_id_fkey FOREIGN KEY (user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_user_notification_states_notification_id_fkey FOREIGN KEY (notification_id)
        REFERENCES hrd_notifications (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_user_notification_states_read_after_delivered_check
        CHECK (read_at IS NULL OR read_at >= delivered_at)
);

CREATE INDEX hrd_user_notification_states_user_created_notification_idx
    ON hrd_user_notification_states (user_id, created_at DESC, notification_id);
CREATE INDEX hrd_user_notification_states_user_unread_created_idx
    ON hrd_user_notification_states (user_id, created_at DESC)
    WHERE read_at IS NULL;

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

-- +goose Down
-- Reverting the job_type CHECK fails if rows already use the newly added types.
-- No silent deletes: down must fail until those rows are removed out-of-band.
ALTER TABLE hrd_background_jobs
    DROP CONSTRAINT hrd_background_jobs_job_type_check;
ALTER TABLE hrd_background_jobs
    ADD CONSTRAINT hrd_background_jobs_job_type_check CHECK (job_type IN (
        'TJK_SYNC_BATCH',
        'MEDIA_VALIDATE_AND_NORMALIZE',
        'MEDIA_GENERATE_VARIANT',
        'MEDIA_DELETE_OBJECTS',
        'MEDIA_RECONCILE'
    ));

DROP TABLE hrd_user_notification_states;
DROP TABLE hrd_notifications;
DROP TABLE hrd_notification_templates;
DROP TABLE hrd_campaigns;
DROP TABLE hrd_advert_feature_activations;
DROP TABLE hrd_advert_package_assignments;
DROP TABLE hrd_packages;
