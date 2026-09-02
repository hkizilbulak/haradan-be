-- +goose Up
-- Satılık Yarış Atı (c1000000-0000-4000-8000-000000000011) için durum özellikleri.
-- İlan verirken switch (aç/kapa) olarak sorulur ve ilan bilgi kısmında gösterilir.

INSERT INTO hrd_category_properties (
    id, category_id, code, title, help_text, data_type,
    is_required, is_public_visible, is_form_visible, is_filterable,
    sort_order, is_active, options, validation, default_value, ui_metadata,
    version, created_at, updated_at
) VALUES
    (
        'f1000000-0000-4000-8000-000000000001',
        'c1000000-0000-4000-8000-000000000011',
        'IN_TRAINING',
        'İdmanda mı',
        'Atın idmanda olup olmadığını belirtiniz',
        'BOOLEAN',
        FALSE, TRUE, TRUE, TRUE,
        14, TRUE,
        '[]'::jsonb,
        '{}'::jsonb, NULL,
        '{"icon": "fitness-outline", "inputWidget": "switch", "displayGroup": "raceStatus"}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'f1000000-0000-4000-8000-000000000003',
        'c1000000-0000-4000-8000-000000000011',
        'IS_RACE_READY',
        'Koşar durumda mı',
        'Atın koşar durumda olup olmadığını belirtiniz',
        'BOOLEAN',
        FALSE, TRUE, TRUE, TRUE,
        15, TRUE,
        '[]'::jsonb,
        '{}'::jsonb, NULL,
        '{"icon": "flash-outline", "inputWidget": "switch", "displayGroup": "raceStatus"}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'f1000000-0000-4000-8000-000000000002',
        'c1000000-0000-4000-8000-000000000011',
        'IS_FOR_RENT',
        'Kiralık mı',
        'Atın kiralık olarak verilip verilmeyeceğini belirtiniz',
        'BOOLEAN',
        FALSE, TRUE, TRUE, TRUE,
        16, TRUE,
        '[]'::jsonb,
        '{}'::jsonb, NULL,
        '{"icon": "key-outline", "inputWidget": "switch", "displayGroup": "raceStatus"}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    )
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    help_text = EXCLUDED.help_text,
    data_type = EXCLUDED.data_type,
    is_required = EXCLUDED.is_required,
    is_public_visible = EXCLUDED.is_public_visible,
    is_form_visible = EXCLUDED.is_form_visible,
    is_filterable = EXCLUDED.is_filterable,
    sort_order = EXCLUDED.sort_order,
    is_active = TRUE,
    options = EXCLUDED.options,
    validation = EXCLUDED.validation,
    default_value = EXCLUDED.default_value,
    ui_metadata = EXCLUDED.ui_metadata,
    updated_at = EXCLUDED.updated_at,
    version = hrd_category_properties.version + 1;

-- +goose Down
DELETE FROM hrd_category_properties WHERE id IN (
    'f1000000-0000-4000-8000-000000000001',
    'f1000000-0000-4000-8000-000000000002',
    'f1000000-0000-4000-8000-000000000003'
);
