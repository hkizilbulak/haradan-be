-- +goose Up
-- Öne çıkan bilgiler (güvence ve inceleme) — Satılık Atlar alt kategorilerine miras alınır.
-- İlan veren checkbox ile seçer; ilan detayında yalnızca işaretlenenler gösterilir.

INSERT INTO hrd_category_properties (
    id, category_id, code, title, help_text, data_type,
    is_required, is_public_visible, is_form_visible, is_filterable,
    sort_order, is_active, options, validation, default_value, ui_metadata,
    version, created_at, updated_at
) VALUES
    (
        'e1000000-0000-4000-8000-000000000001',
        'c1000000-0000-4000-8000-000000000001',
        'HEALTH_VACCINATION',
        'Sağlık & aşı kaydı',
        'Veteriner raporu ve aşı kartı talep edilebilir.',
        'BOOLEAN',
        FALSE, TRUE, TRUE, FALSE,
        20, TRUE, '[]'::jsonb, '{}'::jsonb, NULL,
        '{"displayGroup":"highlight","sectionTitle":"ÖNE ÇIKAN BİLGİLER","sectionSubtitle":"GÜVENCE VE İNCELEME","icon":"activity","inputWidget":"checkbox"}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'e1000000-0000-4000-8000-000000000002',
        'c1000000-0000-4000-8000-000000000001',
        'PEDIGREE_IDENTITY',
        'Şecere ve kimlik',
        'Soy ağacı ve kimlik belgeleri satış öncesi paylaşılır.',
        'BOOLEAN',
        FALSE, TRUE, TRUE, FALSE,
        21, TRUE, '[]'::jsonb, '{}'::jsonb, NULL,
        '{"displayGroup":"highlight","sectionTitle":"ÖNE ÇIKAN BİLGİLER","sectionSubtitle":"GÜVENCE VE İNCELEME","icon":"file-text","inputWidget":"checkbox"}'::jsonb,
        1, TIMESTAMPTZ '2020-01-01 00:00:00+00', TIMESTAMPTZ '2020-01-01 00:00:00+00'
    ),
    (
        'e1000000-0000-4000-8000-000000000003',
        'c1000000-0000-4000-8000-000000000001',
        'ONSITE_INSPECTION',
        'Yerinde inceleme',
        'Deneme binişi ve hara ziyareti randevu ile.',
        'BOOLEAN',
        FALSE, TRUE, TRUE, FALSE,
        22, TRUE, '[]'::jsonb, '{}'::jsonb, NULL,
        '{"displayGroup":"highlight","sectionTitle":"ÖNE ÇIKAN BİLGİLER","sectionSubtitle":"GÜVENCE VE İNCELEME","icon":"eye","inputWidget":"checkbox"}'::jsonb,
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
    is_active = EXCLUDED.is_active,
    ui_metadata = EXCLUDED.ui_metadata,
    updated_at = EXCLUDED.updated_at;

-- +goose Down
DELETE FROM hrd_category_properties
WHERE id IN (
    'e1000000-0000-4000-8000-000000000001',
    'e1000000-0000-4000-8000-000000000002',
    'e1000000-0000-4000-8000-000000000003'
);
