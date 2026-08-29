-- +goose Up
ALTER TABLE hrd_categories ADD COLUMN allow_tjk BOOLEAN NOT NULL DEFAULT false;

UPDATE hrd_categories SET allow_tjk = true WHERE slug IN (
    'satilik-yaris-ati',
    'satilik-kisrak',
    'satilik-aygir',
    'arap-aygir',
    'ingiliz-aygir'
);

-- +goose Down
ALTER TABLE hrd_categories DROP COLUMN IF EXISTS allow_tjk;
