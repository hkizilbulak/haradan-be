-- +goose Up
-- Timed FEATURED entitlement + package featured_days for catalog-driven features.

ALTER TABLE hrd_packages
    ADD COLUMN featured_days integer NULL;

ALTER TABLE hrd_packages
    ADD CONSTRAINT hrd_packages_featured_days_positive_check
        CHECK (featured_days IS NULL OR featured_days > 0);

ALTER TABLE hrd_advert_feature_activations
    ADD COLUMN ends_at timestamptz NULL;

ALTER TABLE hrd_advert_feature_activations
    DROP CONSTRAINT hrd_advert_feature_activations_feature_code_check;

ALTER TABLE hrd_advert_feature_activations
    ADD CONSTRAINT hrd_advert_feature_activations_feature_code_check
        CHECK (feature_code IN ('URGENT', 'FEATURED'));

ALTER TABLE hrd_advert_feature_activations
    ADD CONSTRAINT hrd_advert_feature_activations_ends_at_check
        CHECK (ends_at IS NULL OR ends_at > activated_at);

CREATE INDEX hrd_advert_feature_activations_active_featured_idx
    ON hrd_advert_feature_activations (advert_id)
    WHERE status = 'ACTIVE' AND feature_code = 'FEATURED';

CREATE INDEX hrd_advert_feature_activations_featured_ends_idx
    ON hrd_advert_feature_activations (ends_at)
    WHERE status = 'ACTIVE' AND feature_code = 'FEATURED' AND ends_at IS NOT NULL;

UPDATE hrd_packages SET
    featured_days = NULL,
    updated_at = NOW(),
    version = version + 1
WHERE code = 'STANDARD';

UPDATE hrd_packages SET
    featured_days = 7,
    updated_at = NOW(),
    version = version + 1
WHERE code = 'PREMIUM';

UPDATE hrd_packages SET
    featured_days = 30,
    updated_at = NOW(),
    version = version + 1
WHERE code = 'ULTIMATE';

-- +goose Down
DROP INDEX IF EXISTS hrd_advert_feature_activations_featured_ends_idx;
DROP INDEX IF EXISTS hrd_advert_feature_activations_active_featured_idx;

ALTER TABLE hrd_advert_feature_activations
    DROP CONSTRAINT hrd_advert_feature_activations_ends_at_check;

ALTER TABLE hrd_advert_feature_activations
    DROP CONSTRAINT hrd_advert_feature_activations_feature_code_check;

UPDATE hrd_advert_feature_activations
SET status = 'DEACTIVATED',
    deactivated_at = COALESCE(deactivated_at, NOW()),
    reason = COALESCE(reason, 'FEATURED_ROLLBACK'),
    updated_at = NOW()
WHERE feature_code = 'FEATURED' AND status = 'ACTIVE';

DELETE FROM hrd_advert_feature_activations WHERE feature_code = 'FEATURED';

ALTER TABLE hrd_advert_feature_activations
    ADD CONSTRAINT hrd_advert_feature_activations_feature_code_check
        CHECK (feature_code IN ('URGENT'));

ALTER TABLE hrd_advert_feature_activations
    DROP COLUMN ends_at;

ALTER TABLE hrd_packages
    DROP CONSTRAINT hrd_packages_featured_days_positive_check;

ALTER TABLE hrd_packages
    DROP COLUMN featured_days;
