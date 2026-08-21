-- +goose Up
-- Defensive repair for environments where address/sold_at were marked applied
-- without the physical columns existing (causes POST /v1/me/adverts 500).
ALTER TABLE hrd_adverts ADD COLUMN IF NOT EXISTS address text NULL;
ALTER TABLE hrd_adverts ADD COLUMN IF NOT EXISTS sold_at timestamptz NULL;

CREATE INDEX IF NOT EXISTS hrd_adverts_sold_archive_idx
    ON hrd_adverts (sold_at)
    WHERE status = 'SOLD' AND deleted_at IS NULL;

-- +goose Down
-- Keep columns; prior migrations own the canonical down path.
SELECT 1;
