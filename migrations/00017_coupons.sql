-- +goose Up
CREATE TABLE hrd_coupons (
    id uuid NOT NULL,
    code varchar(64) NOT NULL,
    name varchar(160) NOT NULL,
    discount_type varchar(32) NOT NULL,
    discount_value bigint NOT NULL,
    max_uses integer NULL,
    uses_count integer NOT NULL DEFAULT 0,
    max_uses_per_user integer NOT NULL DEFAULT 1,
    min_spend_amount_minor bigint NULL,
    applicable_package_code varchar(32) NULL,
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_by_user_id uuid NOT NULL,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_coupons_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_coupons_code_key UNIQUE (code),
    CONSTRAINT hrd_coupons_created_by_user_id_fkey FOREIGN KEY (created_by_user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_coupons_discount_type_check CHECK (discount_type IN ('PERCENTAGE', 'FIXED_AMOUNT')),
    CONSTRAINT hrd_coupons_discount_value_positive_check CHECK (discount_value > 0),
    CONSTRAINT hrd_coupons_max_uses_positive_check CHECK (max_uses IS NULL OR max_uses > 0),
    CONSTRAINT hrd_coupons_uses_count_nonnegative_check CHECK (uses_count >= 0),
    CONSTRAINT hrd_coupons_max_uses_per_user_positive_check CHECK (max_uses_per_user > 0),
    CONSTRAINT hrd_coupons_min_spend_nonnegative_check CHECK (min_spend_amount_minor IS NULL OR min_spend_amount_minor >= 0),
    CONSTRAINT hrd_coupons_package_code_check CHECK (applicable_package_code IS NULL OR applicable_package_code IN ('STARTER', 'MIDDLE', 'ADVANCED')),
    CONSTRAINT hrd_coupons_starts_ends_check CHECK (ends_at IS NULL OR starts_at <= ends_at),
    CONSTRAINT hrd_coupons_name_not_blank_check CHECK (btrim(name) <> ''),
    CONSTRAINT hrd_coupons_code_not_blank_check CHECK (btrim(code) <> ''),
    CONSTRAINT hrd_coupons_version_positive_check CHECK (version > 0)
);

CREATE INDEX hrd_coupons_code_active_idx ON hrd_coupons (code, is_active);
CREATE INDEX hrd_coupons_active_window_idx ON hrd_coupons (starts_at, ends_at) WHERE is_active = true;

CREATE TABLE hrd_coupon_usages (
    id uuid NOT NULL,
    coupon_id uuid NOT NULL,
    user_id uuid NOT NULL,
    advert_id uuid NULL,
    discount_amount_minor bigint NOT NULL,
    used_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT hrd_coupon_usages_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_coupon_usages_coupon_id_fkey FOREIGN KEY (coupon_id)
        REFERENCES hrd_coupons (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_coupon_usages_user_id_fkey FOREIGN KEY (user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_coupon_usages_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_coupon_usages_discount_amount_positive_check CHECK (discount_amount_minor > 0)
);

CREATE INDEX hrd_coupon_usages_coupon_user_idx ON hrd_coupon_usages (coupon_id, user_id);
CREATE INDEX hrd_coupon_usages_user_used_at_idx ON hrd_coupon_usages (user_id, used_at DESC);

-- +goose Down
DROP TABLE hrd_coupon_usages;
DROP TABLE hrd_coupons;
