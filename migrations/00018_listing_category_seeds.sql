-- +goose Up
-- Product listing groups (çeşit) and leaf types (tür) for public create-advert.
-- Deterministic IDs. Idempotent on primary key so a replay is a no-op/update.

INSERT INTO hrd_categories (
    id, parent_id, slug, name, description, is_active, sort_order,
    version, created_at, updated_at
) VALUES
    (
        'c1000000-0000-4000-8000-000000000001',
        NULL,
        'satilik-atlar',
        'Satılık Atlar',
        NULL,
        TRUE,
        10,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c1000000-0000-4000-8000-000000000002',
        NULL,
        'at-hizmetleri',
        'At Hizmetleri',
        NULL,
        TRUE,
        20,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c1000000-0000-4000-8000-000000000003',
        NULL,
        'asim-hizmetleri',
        'Aşım Hizmetleri',
        NULL,
        TRUE,
        30,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c1000000-0000-4000-8000-000000000011',
        'c1000000-0000-4000-8000-000000000001',
        'satilik-yaris-ati',
        'Satılık Yarış Atı',
        NULL,
        TRUE,
        10,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c1000000-0000-4000-8000-000000000012',
        'c1000000-0000-4000-8000-000000000001',
        'satilik-kisrak',
        'Satılık Kısrak',
        NULL,
        TRUE,
        20,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c1000000-0000-4000-8000-000000000013',
        'c1000000-0000-4000-8000-000000000001',
        'satilik-aygir',
        'Satılık Aygır',
        NULL,
        TRUE,
        30,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c1000000-0000-4000-8000-000000000014',
        'c1000000-0000-4000-8000-000000000001',
        'satilik-binek-ati',
        'Satılık Binek Atı',
        NULL,
        TRUE,
        40,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c1000000-0000-4000-8000-000000000015',
        'c1000000-0000-4000-8000-000000000001',
        'satilik-pony',
        'Satılık Pony',
        NULL,
        TRUE,
        50,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c1000000-0000-4000-8000-000000000021',
        'c1000000-0000-4000-8000-000000000002',
        'pansiyon-haralar',
        'Pansiyon Haralar',
        NULL,
        TRUE,
        10,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c1000000-0000-4000-8000-000000000022',
        'c1000000-0000-4000-8000-000000000002',
        'at-nakliyesi',
        'At Nakliyesi',
        NULL,
        TRUE,
        20,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c1000000-0000-4000-8000-000000000023',
        'c1000000-0000-4000-8000-000000000002',
        'nalbantlar',
        'Nalbantlar',
        NULL,
        TRUE,
        30,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c1000000-0000-4000-8000-000000000031',
        'c1000000-0000-4000-8000-000000000003',
        'arap-aygir',
        'Arap Aygır',
        NULL,
        TRUE,
        10,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'c1000000-0000-4000-8000-000000000032',
        'c1000000-0000-4000-8000-000000000003',
        'ingiliz-aygir',
        'İngiliz Aygır',
        NULL,
        TRUE,
        20,
        1,
        TIMESTAMPTZ '2020-01-01 00:00:00+00',
        TIMESTAMPTZ '2020-01-01 00:00:00+00'
    )
ON CONFLICT (id) DO UPDATE SET
    parent_id = EXCLUDED.parent_id,
    slug = EXCLUDED.slug,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    is_active = TRUE,
    sort_order = EXCLUDED.sort_order,
    updated_at = EXCLUDED.updated_at,
    version = hrd_categories.version + 1;

-- +goose Down
DELETE FROM hrd_categories WHERE id IN (
    'c1000000-0000-4000-8000-000000000011',
    'c1000000-0000-4000-8000-000000000012',
    'c1000000-0000-4000-8000-000000000013',
    'c1000000-0000-4000-8000-000000000014',
    'c1000000-0000-4000-8000-000000000015',
    'c1000000-0000-4000-8000-000000000021',
    'c1000000-0000-4000-8000-000000000022',
    'c1000000-0000-4000-8000-000000000023',
    'c1000000-0000-4000-8000-000000000031',
    'c1000000-0000-4000-8000-000000000032'
);
DELETE FROM hrd_categories WHERE id IN (
    'c1000000-0000-4000-8000-000000000001',
    'c1000000-0000-4000-8000-000000000002',
    'c1000000-0000-4000-8000-000000000003'
);
