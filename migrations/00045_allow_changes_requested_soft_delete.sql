-- +goose Up
ALTER TABLE hrd_adverts
    DROP CONSTRAINT hrd_adverts_deleted_at_draft_only_check;

ALTER TABLE hrd_adverts
    ADD CONSTRAINT hrd_adverts_deleted_at_draft_only_check
    CHECK (deleted_at IS NULL OR status IN ('DRAFT', 'CHANGES_REQUESTED'));

-- +goose Down
ALTER TABLE hrd_adverts
    DROP CONSTRAINT hrd_adverts_deleted_at_draft_only_check;

ALTER TABLE hrd_adverts
    ADD CONSTRAINT hrd_adverts_deleted_at_draft_only_check
    CHECK (deleted_at IS NULL OR status = 'DRAFT');
