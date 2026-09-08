-- +goose Up
ALTER TABLE hrd_coupons
    DROP CONSTRAINT hrd_coupons_package_code_check;

-- +goose Down
ALTER TABLE hrd_coupons
    ADD CONSTRAINT hrd_coupons_package_code_check
    CHECK (applicable_package_code IS NULL OR applicable_package_code IN ('STARTER', 'MIDDLE', 'ADVANCED'));
