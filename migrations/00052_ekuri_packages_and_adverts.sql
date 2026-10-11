-- +goose Up
-- +goose StatementBegin
ALTER TABLE hrd_packages
    ADD COLUMN package_type varchar(16) NOT NULL DEFAULT 'INDIVIDUAL',
    ADD COLUMN horse_limit integer NULL;

ALTER TABLE hrd_packages
    ADD CONSTRAINT hrd_packages_package_type_check
        CHECK (package_type IN ('INDIVIDUAL', 'EKURI')),
    ADD CONSTRAINT hrd_packages_horse_limit_check
        CHECK (horse_limit IS NULL OR horse_limit > 0);

ALTER TABLE hrd_adverts
    ADD COLUMN ekuri_bundle_id uuid NULL;

CREATE INDEX hrd_adverts_ekuri_bundle_id_idx
    ON hrd_adverts (ekuri_bundle_id)
    WHERE ekuri_bundle_id IS NOT NULL;

INSERT INTO hrd_packages (
    id, code, display_name, description, badge_text, benefits,
    display_price_amount_minor, currency_code, default_duration_days,
    allows_urgent, showcase_eligible, featured_days, search_priority, broadcast_on_publish,
    package_type, horse_limit, is_active, sort_order, version, created_at, updated_at
) VALUES
(
    'a0000000-0000-4000-8000-000000000021',
    'EKURI_3',
    'Eküri 3''lü Paket',
    '3 İlanlık Toplu Eküri Paketi',
    '3 İlan',
    '[
      "in|layers-outline|3 adet at ilanı hakkı",
      "in|time-outline|45 gün yayın",
      "in|images-outline|Her at için 5 görsel + kapak",
      "in|ribbon-outline|Eküri rozeti ve çapraz ilan vitrini",
      "out|star-outline|Öne çıkan ilan",
      "out|trophy-outline|Anasayfa vitrini"
    ]'::jsonb,
    60000,
    'TRY',
    45,
    false,
    false,
    NULL,
    15,
    false,
    'EKURI',
    3,
    true,
    40,
    1,
    NOW(),
    NOW()
),
(
    'a0000000-0000-4000-8000-000000000022',
    'EKURI_5',
    'Eküri 5''li Paket',
    '5 İlanlık Toplu Eküri Paketi',
    '5 İlan',
    '[
      "in|layers-outline|5 adet at ilanı hakkı",
      "in|time-outline|60 gün yayın",
      "in|images-outline|Her at için 5 görsel + kapak",
      "in|ribbon-outline|Eküri rozeti ve çapraz ilan vitrini",
      "in|flash-outline|Acil ilan rozeti",
      "in|star-outline|Öne çıkan ilan · 7 gün",
      "out|trophy-outline|Anasayfa vitrini"
    ]'::jsonb,
    95000,
    'TRY',
    60,
    true,
    false,
    7,
    55,
    false,
    'EKURI',
    5,
    true,
    50,
    1,
    NOW(),
    NOW()
),
(
    'a0000000-0000-4000-8000-000000000023',
    'EKURI_10',
    'Eküri 10''lu Paket',
    '10 İlanlık Kapsamlı Eküri Paketi',
    '10 İlan',
    '[
      "in|layers-outline|10 adet at ilanı hakkı",
      "in|time-outline|90 gün yayın",
      "in|images-outline|Her at için 5 görsel + kapak",
      "in|ribbon-outline|Eküri rozeti ve çapraz ilan vitrini",
      "in|flash-outline|Acil ilan rozeti",
      "in|star-outline|Öne çıkan ilan · 30 gün",
      "in|trophy-outline|Anasayfa vitrini",
      "in|headset-outline|Öncelikli hara desteği"
    ]'::jsonb,
    175000,
    'TRY',
    90,
    true,
    true,
    30,
    100,
    true,
    'EKURI',
    10,
    true,
    60,
    1,
    NOW(),
    NOW()
)
ON CONFLICT (code) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    badge_text = EXCLUDED.badge_text,
    benefits = EXCLUDED.benefits,
    display_price_amount_minor = EXCLUDED.display_price_amount_minor,
    currency_code = EXCLUDED.currency_code,
    default_duration_days = EXCLUDED.default_duration_days,
    allows_urgent = EXCLUDED.allows_urgent,
    showcase_eligible = EXCLUDED.showcase_eligible,
    featured_days = EXCLUDED.featured_days,
    search_priority = EXCLUDED.search_priority,
    broadcast_on_publish = EXCLUDED.broadcast_on_publish,
    package_type = EXCLUDED.package_type,
    horse_limit = EXCLUDED.horse_limit,
    is_active = true,
    sort_order = EXCLUDED.sort_order,
    updated_at = NOW(),
    version = hrd_packages.version + 1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM hrd_packages WHERE code IN ('EKURI_3', 'EKURI_5', 'EKURI_10');
DROP INDEX IF EXISTS hrd_adverts_ekuri_bundle_id_idx;
ALTER TABLE hrd_adverts DROP COLUMN IF EXISTS ekuri_bundle_id;
ALTER TABLE hrd_packages DROP CONSTRAINT IF EXISTS hrd_packages_horse_limit_check;
ALTER TABLE hrd_packages DROP CONSTRAINT IF EXISTS hrd_packages_package_type_check;
ALTER TABLE hrd_packages DROP COLUMN IF EXISTS horse_limit;
ALTER TABLE hrd_packages DROP COLUMN IF EXISTS package_type;
-- +goose StatementEnd
