-- +goose Up
CREATE TABLE hrd_banners (
    id uuid NOT NULL,
    placement varchar(32) NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'INACTIVE',
    asset_id uuid NOT NULL,
    title varchar(160) NULL,
    alt_text varchar(255) NULL,
    target_url text NULL,
    sort_order integer NOT NULL DEFAULT 0,
    version integer NOT NULL DEFAULT 1,
    created_by_user_id uuid NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_banners_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_banners_asset_id_fkey FOREIGN KEY (asset_id)
        REFERENCES hrd_media_assets (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_banners_created_by_user_id_fkey FOREIGN KEY (created_by_user_id)
        REFERENCES hrd_users (id) ON DELETE SET NULL,
    CONSTRAINT hrd_banners_placement_check
        CHECK (placement IN ('HOMEPAGE', 'LISTING_DETAIL', 'SEARCH')),
    CONSTRAINT hrd_banners_status_check CHECK (status IN ('ACTIVE', 'INACTIVE')),
    CONSTRAINT hrd_banners_sort_order_nonnegative_check CHECK (sort_order >= 0),
    CONSTRAINT hrd_banners_version_positive_check CHECK (version > 0)
);

CREATE INDEX hrd_banners_active_placement_sort_idx
    ON hrd_banners (placement, sort_order)
    WHERE status = 'ACTIVE';
CREATE INDEX hrd_banners_asset_id_idx ON hrd_banners (asset_id);

-- +goose Down
DROP TABLE hrd_banners;
