-- +goose Up
-- Migrate hrd_adverts.id from UUID to BIGINT auto-increment, preserving data via legacy map.

CREATE SEQUENCE hrd_adverts_id_seq;

CREATE TABLE hrd_advert_legacy_id_map (
    id bigint NOT NULL,
    legacy_uuid uuid NOT NULL,
    CONSTRAINT hrd_advert_legacy_id_map_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_advert_legacy_id_map_legacy_uuid_key UNIQUE (legacy_uuid)
);

INSERT INTO hrd_advert_legacy_id_map (id, legacy_uuid)
SELECT nextval('hrd_adverts_id_seq'), id
FROM hrd_adverts
ORDER BY created_at ASC, id ASC;

SELECT setval(
    'hrd_adverts_id_seq',
    COALESCE((SELECT MAX(id) FROM hrd_advert_legacy_id_map), 1),
    (SELECT COUNT(*) > 0 FROM hrd_advert_legacy_id_map)
);

-- Drop FK constraints referencing hrd_adverts.id
ALTER TABLE hrd_advert_status_history DROP CONSTRAINT hrd_advert_status_history_advert_id_fkey;
ALTER TABLE hrd_favorites DROP CONSTRAINT hrd_favorites_advert_id_fkey;
ALTER TABLE hrd_advert_media DROP CONSTRAINT hrd_advert_media_advert_id_fkey;
ALTER TABLE hrd_advert_package_assignments DROP CONSTRAINT hrd_advert_package_assignments_advert_id_fkey;
ALTER TABLE hrd_advert_feature_activations DROP CONSTRAINT hrd_advert_feature_activations_advert_id_fkey;
ALTER TABLE hrd_notifications DROP CONSTRAINT hrd_notifications_advert_id_fkey;
ALTER TABLE hrd_coupon_usages DROP CONSTRAINT hrd_coupon_usages_advert_id_fkey;
ALTER TABLE hrd_advert_comments DROP CONSTRAINT hrd_advert_comments_advert_id_fkey;
ALTER TABLE hrd_advert_views DROP CONSTRAINT hrd_advert_views_advert_id_fkey;
ALTER TABLE hrd_paytr_charges DROP CONSTRAINT hrd_paytr_charges_advert_id_fkey;

-- Drop indexes/constraints that reference advert_id (uuid)
DROP INDEX IF EXISTS hrd_advert_status_history_advert_created_idx;
ALTER TABLE hrd_favorites DROP CONSTRAINT hrd_favorites_user_id_advert_id_key;
DROP INDEX IF EXISTS hrd_favorites_advert_id_idx;
ALTER TABLE hrd_advert_media DROP CONSTRAINT hrd_advert_media_advert_id_asset_id_key;
ALTER TABLE hrd_advert_media DROP CONSTRAINT hrd_advert_media_advert_id_display_order_key;
DROP INDEX IF EXISTS hrd_advert_media_one_cover_key;
DROP INDEX IF EXISTS hrd_advert_package_assignments_one_active_per_advert_key;
DROP INDEX IF EXISTS hrd_advert_package_assignments_advert_assigned_idx;
DROP INDEX IF EXISTS hrd_advert_package_assignments_active_package_idx;
DROP INDEX IF EXISTS hrd_advert_feature_activations_one_active_feature_key;
DROP INDEX IF EXISTS hrd_advert_feature_activations_active_urgent_idx;
DROP INDEX IF EXISTS hrd_advert_feature_activations_active_featured_idx;
DROP INDEX IF EXISTS hrd_advert_feature_activations_featured_ends_idx;
DROP INDEX IF EXISTS hrd_idx_advert_comments_lookup;
ALTER TABLE hrd_advert_views DROP CONSTRAINT hrd_advert_views_pkey;
DROP INDEX IF EXISTS hrd_advert_views_advert_id_idx;
DROP INDEX IF EXISTS hrd_paytr_charges_advert_created_idx;

-- Drop hrd_adverts indexes that include id
DROP INDEX IF EXISTS hrd_adverts_public_newest_idx;
DROP INDEX IF EXISTS hrd_adverts_public_category_newest_idx;
DROP INDEX IF EXISTS hrd_adverts_public_district_newest_idx;
DROP INDEX IF EXISTS hrd_adverts_public_horse_newest_idx;

-- Migrate hrd_adverts primary key
ALTER TABLE hrd_adverts ADD COLUMN id_bigint bigint;
UPDATE hrd_adverts a
SET id_bigint = m.id
FROM hrd_advert_legacy_id_map m
WHERE a.id = m.legacy_uuid;
ALTER TABLE hrd_adverts ALTER COLUMN id_bigint SET NOT NULL;
ALTER TABLE hrd_adverts DROP CONSTRAINT hrd_adverts_pkey;
ALTER TABLE hrd_adverts DROP COLUMN id;
ALTER TABLE hrd_adverts RENAME COLUMN id_bigint TO id;
ALTER TABLE hrd_adverts ADD CONSTRAINT hrd_adverts_pkey PRIMARY KEY (id);
ALTER SEQUENCE hrd_adverts_id_seq OWNED BY hrd_adverts.id;
ALTER TABLE hrd_adverts ALTER COLUMN id SET DEFAULT nextval('hrd_adverts_id_seq');

-- Migrate child advert_id columns
ALTER TABLE hrd_advert_status_history ADD COLUMN advert_id_bigint bigint;
UPDATE hrd_advert_status_history h
SET advert_id_bigint = m.id
FROM hrd_advert_legacy_id_map m
WHERE h.advert_id = m.legacy_uuid;
ALTER TABLE hrd_advert_status_history ALTER COLUMN advert_id_bigint SET NOT NULL;
ALTER TABLE hrd_advert_status_history DROP COLUMN advert_id;
ALTER TABLE hrd_advert_status_history RENAME COLUMN advert_id_bigint TO advert_id;

ALTER TABLE hrd_favorites ADD COLUMN advert_id_bigint bigint;
UPDATE hrd_favorites f
SET advert_id_bigint = m.id
FROM hrd_advert_legacy_id_map m
WHERE f.advert_id = m.legacy_uuid;
ALTER TABLE hrd_favorites ALTER COLUMN advert_id_bigint SET NOT NULL;
ALTER TABLE hrd_favorites DROP COLUMN advert_id;
ALTER TABLE hrd_favorites RENAME COLUMN advert_id_bigint TO advert_id;

ALTER TABLE hrd_advert_media ADD COLUMN advert_id_bigint bigint;
UPDATE hrd_advert_media am
SET advert_id_bigint = m.id
FROM hrd_advert_legacy_id_map m
WHERE am.advert_id = m.legacy_uuid;
ALTER TABLE hrd_advert_media ALTER COLUMN advert_id_bigint SET NOT NULL;
ALTER TABLE hrd_advert_media DROP COLUMN advert_id;
ALTER TABLE hrd_advert_media RENAME COLUMN advert_id_bigint TO advert_id;

ALTER TABLE hrd_advert_package_assignments ADD COLUMN advert_id_bigint bigint;
UPDATE hrd_advert_package_assignments apa
SET advert_id_bigint = m.id
FROM hrd_advert_legacy_id_map m
WHERE apa.advert_id = m.legacy_uuid;
ALTER TABLE hrd_advert_package_assignments ALTER COLUMN advert_id_bigint SET NOT NULL;
ALTER TABLE hrd_advert_package_assignments DROP COLUMN advert_id;
ALTER TABLE hrd_advert_package_assignments RENAME COLUMN advert_id_bigint TO advert_id;

ALTER TABLE hrd_advert_feature_activations ADD COLUMN advert_id_bigint bigint;
UPDATE hrd_advert_feature_activations afa
SET advert_id_bigint = m.id
FROM hrd_advert_legacy_id_map m
WHERE afa.advert_id = m.legacy_uuid;
ALTER TABLE hrd_advert_feature_activations ALTER COLUMN advert_id_bigint SET NOT NULL;
ALTER TABLE hrd_advert_feature_activations DROP COLUMN advert_id;
ALTER TABLE hrd_advert_feature_activations RENAME COLUMN advert_id_bigint TO advert_id;

ALTER TABLE hrd_notifications ADD COLUMN advert_id_bigint bigint;
UPDATE hrd_notifications n
SET advert_id_bigint = m.id
FROM hrd_advert_legacy_id_map m
WHERE n.advert_id = m.legacy_uuid;
ALTER TABLE hrd_notifications DROP COLUMN advert_id;
ALTER TABLE hrd_notifications RENAME COLUMN advert_id_bigint TO advert_id;

ALTER TABLE hrd_coupon_usages ADD COLUMN advert_id_bigint bigint;
UPDATE hrd_coupon_usages cu
SET advert_id_bigint = m.id
FROM hrd_advert_legacy_id_map m
WHERE cu.advert_id = m.legacy_uuid;
ALTER TABLE hrd_coupon_usages DROP COLUMN advert_id;
ALTER TABLE hrd_coupon_usages RENAME COLUMN advert_id_bigint TO advert_id;

ALTER TABLE hrd_advert_comments ADD COLUMN advert_id_bigint bigint;
UPDATE hrd_advert_comments ac
SET advert_id_bigint = m.id
FROM hrd_advert_legacy_id_map m
WHERE ac.advert_id = m.legacy_uuid;
ALTER TABLE hrd_advert_comments ALTER COLUMN advert_id_bigint SET NOT NULL;
ALTER TABLE hrd_advert_comments DROP COLUMN advert_id;
ALTER TABLE hrd_advert_comments RENAME COLUMN advert_id_bigint TO advert_id;

ALTER TABLE hrd_advert_views ADD COLUMN advert_id_bigint bigint;
UPDATE hrd_advert_views av
SET advert_id_bigint = m.id
FROM hrd_advert_legacy_id_map m
WHERE av.advert_id = m.legacy_uuid;
ALTER TABLE hrd_advert_views ALTER COLUMN advert_id_bigint SET NOT NULL;
ALTER TABLE hrd_advert_views DROP COLUMN advert_id;
ALTER TABLE hrd_advert_views RENAME COLUMN advert_id_bigint TO advert_id;

ALTER TABLE hrd_paytr_charges ADD COLUMN advert_id_bigint bigint;
UPDATE hrd_paytr_charges pc
SET advert_id_bigint = m.id
FROM hrd_advert_legacy_id_map m
WHERE pc.advert_id = m.legacy_uuid;
ALTER TABLE hrd_paytr_charges ALTER COLUMN advert_id_bigint SET NOT NULL;
ALTER TABLE hrd_paytr_charges DROP COLUMN advert_id;
ALTER TABLE hrd_paytr_charges RENAME COLUMN advert_id_bigint TO advert_id;

-- Recreate FK constraints
ALTER TABLE hrd_advert_status_history
    ADD CONSTRAINT hrd_advert_status_history_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_favorites
    ADD CONSTRAINT hrd_favorites_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_advert_media
    ADD CONSTRAINT hrd_advert_media_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_advert_package_assignments
    ADD CONSTRAINT hrd_advert_package_assignments_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_advert_feature_activations
    ADD CONSTRAINT hrd_advert_feature_activations_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_notifications
    ADD CONSTRAINT hrd_notifications_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_coupon_usages
    ADD CONSTRAINT hrd_coupon_usages_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_advert_comments
    ADD CONSTRAINT hrd_advert_comments_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE CASCADE;
ALTER TABLE hrd_advert_views
    ADD CONSTRAINT hrd_advert_views_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE CASCADE;
ALTER TABLE hrd_paytr_charges
    ADD CONSTRAINT hrd_paytr_charges_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;

-- Recreate indexes
CREATE INDEX hrd_advert_status_history_advert_created_idx
    ON hrd_advert_status_history (advert_id, created_at);
ALTER TABLE hrd_favorites
    ADD CONSTRAINT hrd_favorites_user_id_advert_id_key UNIQUE (user_id, advert_id);
CREATE INDEX hrd_favorites_advert_id_idx ON hrd_favorites (advert_id);
ALTER TABLE hrd_advert_media
    ADD CONSTRAINT hrd_advert_media_advert_id_asset_id_key UNIQUE (advert_id, asset_id);
ALTER TABLE hrd_advert_media
    ADD CONSTRAINT hrd_advert_media_advert_id_display_order_key UNIQUE (advert_id, display_order);
CREATE UNIQUE INDEX hrd_advert_media_one_cover_key
    ON hrd_advert_media (advert_id)
    WHERE is_cover = true;
CREATE UNIQUE INDEX hrd_advert_package_assignments_one_active_per_advert_key
    ON hrd_advert_package_assignments (advert_id)
    WHERE status = 'ACTIVE';
CREATE INDEX hrd_advert_package_assignments_advert_assigned_idx
    ON hrd_advert_package_assignments (advert_id, assigned_at DESC, id DESC);
CREATE INDEX hrd_advert_package_assignments_active_package_idx
    ON hrd_advert_package_assignments (package_id, advert_id)
    WHERE status = 'ACTIVE';
CREATE UNIQUE INDEX hrd_advert_feature_activations_one_active_feature_key
    ON hrd_advert_feature_activations (advert_id, feature_code)
    WHERE status = 'ACTIVE';
CREATE INDEX hrd_advert_feature_activations_active_urgent_idx
    ON hrd_advert_feature_activations (advert_id)
    WHERE status = 'ACTIVE' AND feature_code = 'URGENT';
CREATE INDEX hrd_advert_feature_activations_active_featured_idx
    ON hrd_advert_feature_activations (advert_id)
    WHERE status = 'ACTIVE' AND feature_code = 'FEATURED';
CREATE INDEX hrd_advert_feature_activations_featured_ends_idx
    ON hrd_advert_feature_activations (ends_at)
    WHERE status = 'ACTIVE' AND feature_code = 'FEATURED' AND ends_at IS NOT NULL;
CREATE INDEX hrd_idx_advert_comments_lookup
    ON hrd_advert_comments (advert_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL AND status = 'PUBLISHED';
ALTER TABLE hrd_advert_views
    ADD CONSTRAINT hrd_advert_views_pkey PRIMARY KEY (advert_id, ip_address);
CREATE INDEX hrd_advert_views_advert_id_idx ON hrd_advert_views (advert_id);
CREATE INDEX hrd_paytr_charges_advert_created_idx
    ON hrd_paytr_charges (advert_id, created_at DESC, id DESC);
CREATE INDEX hrd_adverts_public_newest_idx
    ON hrd_adverts (published_at DESC, id DESC)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
CREATE INDEX hrd_adverts_public_category_newest_idx
    ON hrd_adverts (category_id, published_at DESC, id DESC)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
CREATE INDEX hrd_adverts_public_district_newest_idx
    ON hrd_adverts (district_id, published_at DESC, id DESC)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
CREATE INDEX hrd_adverts_public_horse_newest_idx
    ON hrd_adverts (horse_id, published_at DESC, id DESC)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL AND horse_id IS NOT NULL;

-- +goose Down
-- Reverse BIGINT -> UUID using hrd_advert_legacy_id_map (must exist from Up).

ALTER TABLE hrd_advert_status_history DROP CONSTRAINT hrd_advert_status_history_advert_id_fkey;
ALTER TABLE hrd_favorites DROP CONSTRAINT hrd_favorites_advert_id_fkey;
ALTER TABLE hrd_advert_media DROP CONSTRAINT hrd_advert_media_advert_id_fkey;
ALTER TABLE hrd_advert_package_assignments DROP CONSTRAINT hrd_advert_package_assignments_advert_id_fkey;
ALTER TABLE hrd_advert_feature_activations DROP CONSTRAINT hrd_advert_feature_activations_advert_id_fkey;
ALTER TABLE hrd_notifications DROP CONSTRAINT hrd_notifications_advert_id_fkey;
ALTER TABLE hrd_coupon_usages DROP CONSTRAINT hrd_coupon_usages_advert_id_fkey;
ALTER TABLE hrd_advert_comments DROP CONSTRAINT hrd_advert_comments_advert_id_fkey;
ALTER TABLE hrd_advert_views DROP CONSTRAINT hrd_advert_views_advert_id_fkey;
ALTER TABLE hrd_paytr_charges DROP CONSTRAINT hrd_paytr_charges_advert_id_fkey;

DROP INDEX IF EXISTS hrd_advert_status_history_advert_created_idx;
ALTER TABLE hrd_favorites DROP CONSTRAINT hrd_favorites_user_id_advert_id_key;
DROP INDEX IF EXISTS hrd_favorites_advert_id_idx;
ALTER TABLE hrd_advert_media DROP CONSTRAINT hrd_advert_media_advert_id_asset_id_key;
ALTER TABLE hrd_advert_media DROP CONSTRAINT hrd_advert_media_advert_id_display_order_key;
DROP INDEX IF EXISTS hrd_advert_media_one_cover_key;
DROP INDEX IF EXISTS hrd_advert_package_assignments_one_active_per_advert_key;
DROP INDEX IF EXISTS hrd_advert_package_assignments_advert_assigned_idx;
DROP INDEX IF EXISTS hrd_advert_package_assignments_active_package_idx;
DROP INDEX IF EXISTS hrd_advert_feature_activations_one_active_feature_key;
DROP INDEX IF EXISTS hrd_advert_feature_activations_active_urgent_idx;
DROP INDEX IF EXISTS hrd_advert_feature_activations_active_featured_idx;
DROP INDEX IF EXISTS hrd_advert_feature_activations_featured_ends_idx;
DROP INDEX IF EXISTS hrd_idx_advert_comments_lookup;
ALTER TABLE hrd_advert_views DROP CONSTRAINT hrd_advert_views_pkey;
DROP INDEX IF EXISTS hrd_advert_views_advert_id_idx;
DROP INDEX IF EXISTS hrd_paytr_charges_advert_created_idx;
DROP INDEX IF EXISTS hrd_adverts_public_newest_idx;
DROP INDEX IF EXISTS hrd_adverts_public_category_newest_idx;
DROP INDEX IF EXISTS hrd_adverts_public_district_newest_idx;
DROP INDEX IF EXISTS hrd_adverts_public_horse_newest_idx;

ALTER TABLE hrd_adverts ALTER COLUMN id DROP DEFAULT;
ALTER SEQUENCE hrd_adverts_id_seq OWNED BY NONE;

ALTER TABLE hrd_adverts ADD COLUMN id_uuid uuid;
UPDATE hrd_adverts a
SET id_uuid = m.legacy_uuid
FROM hrd_advert_legacy_id_map m
WHERE a.id = m.id;
ALTER TABLE hrd_adverts ALTER COLUMN id_uuid SET NOT NULL;
ALTER TABLE hrd_adverts DROP CONSTRAINT hrd_adverts_pkey;
ALTER TABLE hrd_adverts DROP COLUMN id;
ALTER TABLE hrd_adverts RENAME COLUMN id_uuid TO id;
ALTER TABLE hrd_adverts ADD CONSTRAINT hrd_adverts_pkey PRIMARY KEY (id);

ALTER TABLE hrd_advert_status_history ADD COLUMN advert_id_uuid uuid;
UPDATE hrd_advert_status_history h
SET advert_id_uuid = m.legacy_uuid
FROM hrd_advert_legacy_id_map m
WHERE h.advert_id = m.id;
ALTER TABLE hrd_advert_status_history ALTER COLUMN advert_id_uuid SET NOT NULL;
ALTER TABLE hrd_advert_status_history DROP COLUMN advert_id;
ALTER TABLE hrd_advert_status_history RENAME COLUMN advert_id_uuid TO advert_id;

ALTER TABLE hrd_favorites ADD COLUMN advert_id_uuid uuid;
UPDATE hrd_favorites f
SET advert_id_uuid = m.legacy_uuid
FROM hrd_advert_legacy_id_map m
WHERE f.advert_id = m.id;
ALTER TABLE hrd_favorites ALTER COLUMN advert_id_uuid SET NOT NULL;
ALTER TABLE hrd_favorites DROP COLUMN advert_id;
ALTER TABLE hrd_favorites RENAME COLUMN advert_id_uuid TO advert_id;

ALTER TABLE hrd_advert_media ADD COLUMN advert_id_uuid uuid;
UPDATE hrd_advert_media am
SET advert_id_uuid = m.legacy_uuid
FROM hrd_advert_legacy_id_map m
WHERE am.advert_id = m.id;
ALTER TABLE hrd_advert_media ALTER COLUMN advert_id_uuid SET NOT NULL;
ALTER TABLE hrd_advert_media DROP COLUMN advert_id;
ALTER TABLE hrd_advert_media RENAME COLUMN advert_id_uuid TO advert_id;

ALTER TABLE hrd_advert_package_assignments ADD COLUMN advert_id_uuid uuid;
UPDATE hrd_advert_package_assignments apa
SET advert_id_uuid = m.legacy_uuid
FROM hrd_advert_legacy_id_map m
WHERE apa.advert_id = m.id;
ALTER TABLE hrd_advert_package_assignments ALTER COLUMN advert_id_uuid SET NOT NULL;
ALTER TABLE hrd_advert_package_assignments DROP COLUMN advert_id;
ALTER TABLE hrd_advert_package_assignments RENAME COLUMN advert_id_uuid TO advert_id;

ALTER TABLE hrd_advert_feature_activations ADD COLUMN advert_id_uuid uuid;
UPDATE hrd_advert_feature_activations afa
SET advert_id_uuid = m.legacy_uuid
FROM hrd_advert_legacy_id_map m
WHERE afa.advert_id = m.id;
ALTER TABLE hrd_advert_feature_activations ALTER COLUMN advert_id_uuid SET NOT NULL;
ALTER TABLE hrd_advert_feature_activations DROP COLUMN advert_id;
ALTER TABLE hrd_advert_feature_activations RENAME COLUMN advert_id_uuid TO advert_id;

ALTER TABLE hrd_notifications ADD COLUMN advert_id_uuid uuid;
UPDATE hrd_notifications n
SET advert_id_uuid = m.legacy_uuid
FROM hrd_advert_legacy_id_map m
WHERE n.advert_id = m.id;
ALTER TABLE hrd_notifications DROP COLUMN advert_id;
ALTER TABLE hrd_notifications RENAME COLUMN advert_id_uuid TO advert_id;

ALTER TABLE hrd_coupon_usages ADD COLUMN advert_id_uuid uuid;
UPDATE hrd_coupon_usages cu
SET advert_id_uuid = m.legacy_uuid
FROM hrd_advert_legacy_id_map m
WHERE cu.advert_id = m.id;
ALTER TABLE hrd_coupon_usages DROP COLUMN advert_id;
ALTER TABLE hrd_coupon_usages RENAME COLUMN advert_id_uuid TO advert_id;

ALTER TABLE hrd_advert_comments ADD COLUMN advert_id_uuid uuid;
UPDATE hrd_advert_comments ac
SET advert_id_uuid = m.legacy_uuid
FROM hrd_advert_legacy_id_map m
WHERE ac.advert_id = m.id;
ALTER TABLE hrd_advert_comments ALTER COLUMN advert_id_uuid SET NOT NULL;
ALTER TABLE hrd_advert_comments DROP COLUMN advert_id;
ALTER TABLE hrd_advert_comments RENAME COLUMN advert_id_uuid TO advert_id;

ALTER TABLE hrd_advert_views ADD COLUMN advert_id_uuid uuid;
UPDATE hrd_advert_views av
SET advert_id_uuid = m.legacy_uuid
FROM hrd_advert_legacy_id_map m
WHERE av.advert_id = m.id;
ALTER TABLE hrd_advert_views ALTER COLUMN advert_id_uuid SET NOT NULL;
ALTER TABLE hrd_advert_views DROP COLUMN advert_id;
ALTER TABLE hrd_advert_views RENAME COLUMN advert_id_uuid TO advert_id;

ALTER TABLE hrd_paytr_charges ADD COLUMN advert_id_uuid uuid;
UPDATE hrd_paytr_charges pc
SET advert_id_uuid = m.legacy_uuid
FROM hrd_advert_legacy_id_map m
WHERE pc.advert_id = m.id;
ALTER TABLE hrd_paytr_charges ALTER COLUMN advert_id_uuid SET NOT NULL;
ALTER TABLE hrd_paytr_charges DROP COLUMN advert_id;
ALTER TABLE hrd_paytr_charges RENAME COLUMN advert_id_uuid TO advert_id;

ALTER TABLE hrd_advert_status_history
    ADD CONSTRAINT hrd_advert_status_history_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_favorites
    ADD CONSTRAINT hrd_favorites_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_advert_media
    ADD CONSTRAINT hrd_advert_media_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_advert_package_assignments
    ADD CONSTRAINT hrd_advert_package_assignments_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_advert_feature_activations
    ADD CONSTRAINT hrd_advert_feature_activations_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_notifications
    ADD CONSTRAINT hrd_notifications_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_coupon_usages
    ADD CONSTRAINT hrd_coupon_usages_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;
ALTER TABLE hrd_advert_comments
    ADD CONSTRAINT hrd_advert_comments_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE CASCADE;
ALTER TABLE hrd_advert_views
    ADD CONSTRAINT hrd_advert_views_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE CASCADE;
ALTER TABLE hrd_paytr_charges
    ADD CONSTRAINT hrd_paytr_charges_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT;

CREATE INDEX hrd_advert_status_history_advert_created_idx
    ON hrd_advert_status_history (advert_id, created_at);
ALTER TABLE hrd_favorites
    ADD CONSTRAINT hrd_favorites_user_id_advert_id_key UNIQUE (user_id, advert_id);
CREATE INDEX hrd_favorites_advert_id_idx ON hrd_favorites (advert_id);
ALTER TABLE hrd_advert_media
    ADD CONSTRAINT hrd_advert_media_advert_id_asset_id_key UNIQUE (advert_id, asset_id);
ALTER TABLE hrd_advert_media
    ADD CONSTRAINT hrd_advert_media_advert_id_display_order_key UNIQUE (advert_id, display_order);
CREATE UNIQUE INDEX hrd_advert_media_one_cover_key
    ON hrd_advert_media (advert_id)
    WHERE is_cover = true;
CREATE UNIQUE INDEX hrd_advert_package_assignments_one_active_per_advert_key
    ON hrd_advert_package_assignments (advert_id)
    WHERE status = 'ACTIVE';
CREATE INDEX hrd_advert_package_assignments_advert_assigned_idx
    ON hrd_advert_package_assignments (advert_id, assigned_at DESC, id DESC);
CREATE INDEX hrd_advert_package_assignments_active_package_idx
    ON hrd_advert_package_assignments (package_id, advert_id)
    WHERE status = 'ACTIVE';
CREATE UNIQUE INDEX hrd_advert_feature_activations_one_active_feature_key
    ON hrd_advert_feature_activations (advert_id, feature_code)
    WHERE status = 'ACTIVE';
CREATE INDEX hrd_advert_feature_activations_active_urgent_idx
    ON hrd_advert_feature_activations (advert_id)
    WHERE status = 'ACTIVE' AND feature_code = 'URGENT';
CREATE INDEX hrd_advert_feature_activations_active_featured_idx
    ON hrd_advert_feature_activations (advert_id)
    WHERE status = 'ACTIVE' AND feature_code = 'FEATURED';
CREATE INDEX hrd_advert_feature_activations_featured_ends_idx
    ON hrd_advert_feature_activations (ends_at)
    WHERE status = 'ACTIVE' AND feature_code = 'FEATURED' AND ends_at IS NOT NULL;
CREATE INDEX hrd_idx_advert_comments_lookup
    ON hrd_advert_comments (advert_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL AND status = 'PUBLISHED';
ALTER TABLE hrd_advert_views
    ADD CONSTRAINT hrd_advert_views_pkey PRIMARY KEY (advert_id, ip_address);
CREATE INDEX hrd_advert_views_advert_id_idx ON hrd_advert_views (advert_id);
CREATE INDEX hrd_paytr_charges_advert_created_idx
    ON hrd_paytr_charges (advert_id, created_at DESC, id DESC);
CREATE INDEX hrd_adverts_public_newest_idx
    ON hrd_adverts (published_at DESC, id DESC)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
CREATE INDEX hrd_adverts_public_category_newest_idx
    ON hrd_adverts (category_id, published_at DESC, id DESC)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
CREATE INDEX hrd_adverts_public_district_newest_idx
    ON hrd_adverts (district_id, published_at DESC, id DESC)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
CREATE INDEX hrd_adverts_public_horse_newest_idx
    ON hrd_adverts (horse_id, published_at DESC, id DESC)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL AND horse_id IS NOT NULL;

DROP TABLE hrd_advert_legacy_id_map;
DROP SEQUENCE hrd_adverts_id_seq;
