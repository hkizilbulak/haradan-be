-- +goose Up
-- TJK pedigree and identity properties for Satılık Atlar (c1000000-0000-4000-8000-000000000001)
-- and Aşım Hizmetleri (c1000000-0000-4000-8000-000000000003).
-- Inherited automatically by child leaf categories via recursive form query.

INSERT INTO hrd_category_properties (
    id, category_id, code, title, help_text, data_type,
    is_required, is_public_visible, is_form_visible, is_filterable,
    sort_order, is_active, options, validation, default_value, ui_metadata,
    version, created_at, updated_at
) VALUES
    -- Satılık Atlar Pedigree & Identity
    (
        'd1000000-0000-4000-8000-000000000005',
        'c1000000-0000-4000-8000-000000000001',
        'SIRE',
        'Baba (Sire)',
        'Atın babasının adı',
        'STRING',
        FALSE, TRUE, TRUE, FALSE,
        5, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000006',
        'c1000000-0000-4000-8000-000000000001',
        'DAM',
        'Anne (Dam)',
        'Atın annesinin adı',
        'STRING',
        FALSE, TRUE, TRUE, FALSE,
        6, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000007',
        'c1000000-0000-4000-8000-000000000001',
        'DAMSIRE',
        'Annenin Babası (Damsire)',
        'Kısrak babası',
        'STRING',
        FALSE, TRUE, TRUE, FALSE,
        7, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000008',
        'c1000000-0000-4000-8000-000000000001',
        'HEIGHT_CM',
        'Cidago (cm)',
        'Atın cidago yüksekliği (cm)',
        'INTEGER',
        FALSE, TRUE, TRUE, FALSE,
        8, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000009',
        'c1000000-0000-4000-8000-000000000001',
        'BIRTH_DATE',
        'Doğum Tarihi',
        'Atın doğum tarihi (YYYY-AA-GG)',
        'STRING',
        FALSE, TRUE, TRUE, FALSE,
        9, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000010',
        'c1000000-0000-4000-8000-000000000001',
        'BREEDER',
        'Yetiştirici',
        'Atın yetiştiricisi',
        'STRING',
        FALSE, TRUE, TRUE, FALSE,
        10, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000011',
        'c1000000-0000-4000-8000-000000000001',
        'TRAINER',
        'Antrenör',
        'Atın antrenörü',
        'STRING',
        FALSE, TRUE, TRUE, FALSE,
        11, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000012',
        'c1000000-0000-4000-8000-000000000001',
        'TJK_NUMBER',
        'TJK No',
        'TJK tescil veya mikroçip numarası',
        'STRING',
        FALSE, TRUE, TRUE, TRUE,
        12, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000013',
        'c1000000-0000-4000-8000-000000000001',
        'REGISTERED_NAME',
        'Kayıtlı Adı',
        'Atın TJK tescilli adı',
        'STRING',
        FALSE, TRUE, TRUE, TRUE,
        13, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),

    -- Aşım Hizmetleri Pedigree & Identity
    (
        'd1000000-0000-4000-8000-000000000044',
        'c1000000-0000-4000-8000-000000000003',
        'studHorseName',
        'Aygır Adı',
        'Aşım hizmeti sunulan aygırın adı',
        'STRING',
        FALSE, TRUE, TRUE, TRUE,
        4, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000045',
        'c1000000-0000-4000-8000-000000000003',
        'studSire',
        'Baba (Sire)',
        'Aygırın babası',
        'STRING',
        FALSE, TRUE, TRUE, FALSE,
        5, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000046',
        'c1000000-0000-4000-8000-000000000003',
        'studDam',
        'Anne (Dam)',
        'Aygırın annesi',
        'STRING',
        FALSE, TRUE, TRUE, FALSE,
        6, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000047',
        'c1000000-0000-4000-8000-000000000003',
        'studDamsire',
        'Annenin Babası (Damsire)',
        'Kısrak babası',
        'STRING',
        FALSE, TRUE, TRUE, FALSE,
        7, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000048',
        'c1000000-0000-4000-8000-000000000003',
        'TJK_NUMBER',
        'TJK No',
        'TJK tescil numarası',
        'STRING',
        FALSE, TRUE, TRUE, TRUE,
        8, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000049',
        'c1000000-0000-4000-8000-000000000003',
        'BREEDER',
        'Yetiştirici',
        'Aygırın yetiştiricisi',
        'STRING',
        FALSE, TRUE, TRUE, FALSE,
        9, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'd1000000-0000-4000-8000-000000000050',
        'c1000000-0000-4000-8000-000000000003',
        'TRAINER',
        'Antrenör',
        'Aygırın antrenörü',
        'STRING',
        FALSE, TRUE, TRUE, FALSE,
        10, TRUE, '[]'::jsonb, '{}'::jsonb, NULL, '{}'::jsonb,
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
    'd1000000-0000-4000-8000-000000000005',
    'd1000000-0000-4000-8000-000000000006',
    'd1000000-0000-4000-8000-000000000007',
    'd1000000-0000-4000-8000-000000000008',
    'd1000000-0000-4000-8000-000000000009',
    'd1000000-0000-4000-8000-000000000010',
    'd1000000-0000-4000-8000-000000000011',
    'd1000000-0000-4000-8000-000000000012',
    'd1000000-0000-4000-8000-000000000013',
    'd1000000-0000-4000-8000-000000000044',
    'd1000000-0000-4000-8000-000000000045',
    'd1000000-0000-4000-8000-000000000046',
    'd1000000-0000-4000-8000-000000000047',
    'd1000000-0000-4000-8000-000000000048',
    'd1000000-0000-4000-8000-000000000049',
    'd1000000-0000-4000-8000-000000000050'
);
