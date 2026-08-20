-- +goose Up
-- sold_at records when the owner marked the advert as sold.
-- It is set once by the PUBLISHED -> SOLD transition and never updated.
ALTER TABLE hrd_adverts ADD COLUMN sold_at timestamptz NULL;

-- Index used by the auto-archive background job (SOLD rows older than 24 h).
CREATE INDEX hrd_adverts_sold_archive_idx
    ON hrd_adverts (sold_at)
    WHERE status = 'SOLD' AND deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS hrd_adverts_sold_archive_idx;
ALTER TABLE hrd_adverts DROP COLUMN IF EXISTS sold_at;
