-- +goose Up
CREATE TABLE hrd_media_assets (
    id uuid NOT NULL,
    owner_user_id uuid NOT NULL,
    provider varchar(32) NOT NULL DEFAULT 'B2',
    raw_object_key varchar(512) NULL,
    master_object_key varchar(512) NULL,
    content_type varchar(128) NULL,
    byte_size bigint NULL,
    checksum_sha256 varchar(64) NULL,
    width_px integer NULL,
    height_px integer NULL,
    lifecycle_status varchar(32) NOT NULL DEFAULT 'UPLOAD_PENDING',
    technical_metadata jsonb NOT NULL DEFAULT '{}',
    failure_reason text NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_media_assets_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_media_assets_owner_user_id_fkey FOREIGN KEY (owner_user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_media_assets_lifecycle_status_check CHECK (lifecycle_status IN (
        'UPLOAD_PENDING', 'UPLOADED', 'VALIDATING', 'MASTER_READY',
        'VALIDATION_FAILED', 'CLEANUP_CANDIDATE', 'DELETING', 'PHYSICALLY_DELETED'
    )),
    CONSTRAINT hrd_media_assets_technical_metadata_object_check
        CHECK (jsonb_typeof(technical_metadata) = 'object'),
    CONSTRAINT hrd_media_assets_byte_size_check CHECK (byte_size IS NULL OR byte_size >= 0),
    CONSTRAINT hrd_media_assets_dims_positive_check CHECK (
        (width_px IS NULL OR width_px > 0)
        AND (height_px IS NULL OR height_px > 0)
    ),
    CONSTRAINT hrd_media_assets_master_ready_fields_check CHECK (
        lifecycle_status <> 'MASTER_READY'
        OR (
            master_object_key IS NOT NULL
            AND content_type IS NOT NULL
            AND byte_size IS NOT NULL
            AND width_px IS NOT NULL
            AND height_px IS NOT NULL
            AND byte_size >= 0
            AND width_px > 0
            AND height_px > 0
        )
    ),
    CONSTRAINT hrd_media_assets_validation_failed_reason_check CHECK (
        lifecycle_status <> 'VALIDATION_FAILED'
        OR (failure_reason IS NOT NULL AND btrim(failure_reason) <> '')
    ),
    CONSTRAINT hrd_media_assets_uploaded_raw_key_check CHECK (
        lifecycle_status NOT IN ('UPLOADED', 'VALIDATING') OR raw_object_key IS NOT NULL
    ),
    CONSTRAINT hrd_media_assets_raw_object_key_not_blank_check
        CHECK (raw_object_key IS NULL OR btrim(raw_object_key) <> ''),
    CONSTRAINT hrd_media_assets_master_object_key_not_blank_check
        CHECK (master_object_key IS NULL OR btrim(master_object_key) <> '')
);

CREATE UNIQUE INDEX hrd_media_assets_provider_raw_object_key_key
    ON hrd_media_assets (provider, raw_object_key)
    WHERE raw_object_key IS NOT NULL;
CREATE UNIQUE INDEX hrd_media_assets_provider_master_object_key_key
    ON hrd_media_assets (provider, master_object_key)
    WHERE master_object_key IS NOT NULL;
CREATE INDEX hrd_media_assets_owner_created_idx
    ON hrd_media_assets (owner_user_id, created_at DESC);
CREATE INDEX hrd_media_assets_cleanup_idx
    ON hrd_media_assets (lifecycle_status, updated_at)
    WHERE lifecycle_status IN ('CLEANUP_CANDIDATE', 'DELETING');

CREATE TABLE hrd_media_variants (
    id uuid NOT NULL,
    asset_id uuid NOT NULL,
    transform_profile varchar(64) NOT NULL,
    object_key varchar(512) NULL,
    lifecycle_status varchar(32) NOT NULL DEFAULT 'PENDING',
    width_px integer NULL,
    height_px integer NULL,
    byte_size bigint NULL,
    content_type varchar(128) NULL,
    failure_reason text NULL,
    technical_metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_media_variants_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_media_variants_asset_id_fkey FOREIGN KEY (asset_id)
        REFERENCES hrd_media_assets (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_media_variants_asset_id_transform_profile_key
        UNIQUE (asset_id, transform_profile),
    CONSTRAINT hrd_media_variants_lifecycle_status_check CHECK (lifecycle_status IN (
        'PENDING', 'PROCESSING', 'READY', 'FAILED', 'DELETING', 'PHYSICALLY_DELETED'
    )),
    CONSTRAINT hrd_media_variants_technical_metadata_object_check
        CHECK (jsonb_typeof(technical_metadata) = 'object'),
    CONSTRAINT hrd_media_variants_ready_fields_check CHECK (
        lifecycle_status <> 'READY'
        OR (
            object_key IS NOT NULL
            AND content_type IS NOT NULL
            AND byte_size IS NOT NULL
            AND width_px IS NOT NULL
            AND height_px IS NOT NULL
            AND byte_size >= 0
            AND width_px > 0
            AND height_px > 0
        )
    ),
    CONSTRAINT hrd_media_variants_failed_reason_check CHECK (
        lifecycle_status <> 'FAILED'
        OR (failure_reason IS NOT NULL AND btrim(failure_reason) <> '')
    ),
    CONSTRAINT hrd_media_variants_dims_positive_check CHECK (
        (width_px IS NULL OR width_px > 0)
        AND (height_px IS NULL OR height_px > 0)
    ),
    CONSTRAINT hrd_media_variants_byte_size_check CHECK (byte_size IS NULL OR byte_size >= 0),
    CONSTRAINT hrd_media_variants_object_key_not_blank_check
        CHECK (object_key IS NULL OR btrim(object_key) <> ''),
    CONSTRAINT hrd_media_variants_transform_profile_not_blank_check
        CHECK (btrim(transform_profile) <> '')
);

CREATE UNIQUE INDEX hrd_media_variants_object_key_key
    ON hrd_media_variants (object_key)
    WHERE object_key IS NOT NULL;
CREATE INDEX hrd_media_variants_asset_status_idx
    ON hrd_media_variants (asset_id, lifecycle_status);

CREATE TABLE hrd_advert_media (
    id uuid NOT NULL,
    advert_id uuid NOT NULL,
    asset_id uuid NOT NULL,
    display_order integer NOT NULL,
    is_cover boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_advert_media_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_advert_media_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_advert_media_asset_id_fkey FOREIGN KEY (asset_id)
        REFERENCES hrd_media_assets (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_advert_media_advert_id_asset_id_key UNIQUE (advert_id, asset_id),
    CONSTRAINT hrd_advert_media_advert_id_display_order_key UNIQUE (advert_id, display_order),
    CONSTRAINT hrd_advert_media_display_order_nonnegative_check CHECK (display_order >= 0)
);

CREATE UNIQUE INDEX hrd_advert_media_one_cover_key
    ON hrd_advert_media (advert_id)
    WHERE is_cover = true;
CREATE INDEX hrd_advert_media_asset_id_idx ON hrd_advert_media (asset_id);

-- +goose Down
DROP TABLE hrd_advert_media;
DROP TABLE hrd_media_variants;
DROP TABLE hrd_media_assets;
