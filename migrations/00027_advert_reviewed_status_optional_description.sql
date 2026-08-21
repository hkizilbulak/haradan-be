-- +goose Up
-- Application submit validation made description optional. The old DB check still
-- demanded a non-empty description for PENDING_REVIEW+, so TransitionStatus failed
-- with a check violation → HTTP 500 INTERNAL_ERROR on submit.
ALTER TABLE hrd_adverts
    DROP CONSTRAINT IF EXISTS hrd_adverts_reviewed_status_required_fields_check;

ALTER TABLE hrd_adverts
    ADD CONSTRAINT hrd_adverts_reviewed_status_required_fields_check CHECK (
        status NOT IN (
            'PENDING_REVIEW', 'PUBLISHED', 'REJECTED', 'SUSPENDED', 'SOLD', 'ARCHIVED'
        )
        OR (
            category_id IS NOT NULL
            AND district_id IS NOT NULL
            AND title IS NOT NULL
            AND btrim(title) <> ''
        )
    );

-- +goose Down
ALTER TABLE hrd_adverts
    DROP CONSTRAINT IF EXISTS hrd_adverts_reviewed_status_required_fields_check;

ALTER TABLE hrd_adverts
    ADD CONSTRAINT hrd_adverts_reviewed_status_required_fields_check CHECK (
        status NOT IN (
            'PENDING_REVIEW', 'PUBLISHED', 'REJECTED', 'SUSPENDED', 'SOLD', 'ARCHIVED'
        )
        OR (
            category_id IS NOT NULL
            AND district_id IS NOT NULL
            AND title IS NOT NULL
            AND description IS NOT NULL
            AND btrim(title) <> ''
            AND btrim(description) <> ''
        )
    );
