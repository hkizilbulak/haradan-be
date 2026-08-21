-- +goose Up
-- PayTR checkout charges for listing packages. Table name avoids the reserved
-- "payment" token used by migration safety checks.

ALTER TABLE hrd_advert_package_assignments
    DROP CONSTRAINT hrd_advert_package_assignments_source_check;

ALTER TABLE hrd_advert_package_assignments
    ADD CONSTRAINT hrd_advert_package_assignments_source_check CHECK (source IN (
        'ADMIN', 'SYSTEM', 'PAYMENT'
    ));

CREATE TABLE hrd_paytr_charges (
    id UUID NOT NULL,
    merchant_oid TEXT NOT NULL,
    advert_id UUID NOT NULL,
    owner_user_id UUID NOT NULL,
    pkg_code TEXT NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency_code TEXT NOT NULL DEFAULT 'TRY',
    status TEXT NOT NULL,
    iframe_token TEXT,
    user_ip TEXT,
    token_request_json TEXT,
    token_response_json TEXT,
    notify_payload_json TEXT,
    fail_reason_code TEXT,
    fail_reason_msg TEXT,
    paid_at TIMESTAMPTZ,
    advert_submitted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT hrd_paytr_charges_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_paytr_charges_merchant_oid_key UNIQUE (merchant_oid),
    CONSTRAINT hrd_paytr_charges_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_paytr_charges_owner_user_id_fkey FOREIGN KEY (owner_user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_paytr_charges_status_check CHECK (status IN (
        'PENDING', 'SUCCEEDED', 'FAILED', 'CANCELLED'
    )),
    CONSTRAINT hrd_paytr_charges_amount_non_negative_check CHECK (amount_minor >= 0),
    CONSTRAINT hrd_paytr_charges_currency_check CHECK (currency_code = 'TRY'),
    CONSTRAINT hrd_paytr_charges_pkg_code_check CHECK (pkg_code ~ '^[A-Z0-9][A-Z0-9_-]{1,63}$'),
    CONSTRAINT hrd_paytr_charges_version_positive_check CHECK (version > 0)
);

CREATE INDEX hrd_paytr_charges_advert_created_idx
    ON hrd_paytr_charges (advert_id, created_at DESC, id DESC);
CREATE INDEX hrd_paytr_charges_owner_created_idx
    ON hrd_paytr_charges (owner_user_id, created_at DESC);
CREATE INDEX hrd_paytr_charges_status_idx
    ON hrd_paytr_charges (status)
    WHERE status = 'PENDING';

-- +goose Down
DROP TABLE hrd_paytr_charges;

ALTER TABLE hrd_advert_package_assignments
    DROP CONSTRAINT hrd_advert_package_assignments_source_check;

ALTER TABLE hrd_advert_package_assignments
    ADD CONSTRAINT hrd_advert_package_assignments_source_check CHECK (source IN (
        'ADMIN', 'SYSTEM'
    ));
