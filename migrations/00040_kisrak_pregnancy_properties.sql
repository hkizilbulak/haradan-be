-- +goose Up
-- Satılık Kısrak (c1000000-0000-4000-8000-000000000012) için gebelik özellikleri.
-- İlan verirken 'Gebe mi' sorusuna evet denildiğinde aygır, gebelik evresi ve son aşım tarihi istenir.

INSERT INTO hrd_category_properties (
    id, category_id, code, title, help_text, data_type,
    is_required, is_public_visible, is_form_visible, is_filterable,
    sort_order, is_active, options, validation, default_value, ui_metadata,
    version, created_at, updated_at
) VALUES
    (
        'f1000000-0000-4000-8000-000000000021',
        'c1000000-0000-4000-8000-000000000012',
        'IS_PREGNANT',
        'Gebe mi',
        'Kısrağın gebe olup olmadığını belirtiniz',
        'BOOLEAN',
        FALSE, TRUE, TRUE, TRUE,
        14, TRUE,
        '[]'::jsonb,
        '{}'::jsonb, NULL,
        '{"icon": "heart-outline", "inputWidget": "switch", "displayGroup": "pregnancyStatus"}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'f1000000-0000-4000-8000-000000000022',
        'c1000000-0000-4000-8000-000000000012',
        'COVERING_STALLION',
        'Gebe Olduğu Aygır',
        'Kısrağın gebe olduğu aygırın adını giriniz',
        'STRING',
        FALSE, TRUE, TRUE, TRUE,
        15, TRUE,
        '[]'::jsonb,
        '{}'::jsonb, NULL,
        '{"dependsOn": "IS_PREGNANT", "displayGroup": "pregnancyStatus"}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'f1000000-0000-4000-8000-000000000023',
        'c1000000-0000-4000-8000-000000000012',
        'PREGNANCY_STAGE',
        'Gebelik Durumu',
        'Gebelik kontrol durumunu seçiniz (K1, K2, K3)',
        'SINGLE_SELECT',
        FALSE, TRUE, TRUE, TRUE,
        16, TRUE,
        '[{"value": "K1", "label": "K1"}, {"value": "K2", "label": "K2"}, {"value": "K3", "label": "K3"}]'::jsonb,
        '{}'::jsonb, NULL,
        '{"dependsOn": "IS_PREGNANT", "displayGroup": "pregnancyStatus"}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'f1000000-0000-4000-8000-000000000024',
        'c1000000-0000-4000-8000-000000000012',
        'LAST_COVERING_DATE',
        'Son Aşım Tarihi',
        'Son aşım tarihi (GG.AA.YYYY)',
        'STRING',
        FALSE, TRUE, TRUE, FALSE,
        17, TRUE,
        '[]'::jsonb,
        '{}'::jsonb, NULL,
        '{"dependsOn": "IS_PREGNANT", "displayGroup": "pregnancyStatus"}'::jsonb,
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
    'f1000000-0000-4000-8000-000000000021',
    'f1000000-0000-4000-8000-000000000022',
    'f1000000-0000-4000-8000-000000000023',
    'f1000000-0000-4000-8000-000000000024'
);
