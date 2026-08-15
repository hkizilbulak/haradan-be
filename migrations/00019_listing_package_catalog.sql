-- +goose Up
-- Public listing packages matching FE İlan Ver catalog (Standart / Premium / Ultimate).

INSERT INTO hrd_packages (
    id, code, display_name, description, badge_text, benefits,
    display_price_amount_minor, currency_code, default_duration_days,
    allows_urgent, showcase_eligible, search_priority, broadcast_on_publish,
    is_active, sort_order, version, created_at, updated_at
) VALUES
(
    'a0000000-0000-4000-8000-000000000011',
    'STANDARD',
    'Standart',
    'Temel yayın',
    NULL,
    '[
      "in|time-outline|30 gün yayın",
      "in|images-outline|5 görsel + kapak",
      "out|flash-outline|Acil ilan rozeti",
      "out|star-outline|Öne çıkan ilan",
      "out|share-social-outline|Sosyal medyada yayın",
      "out|trophy-outline|Anasayfa vitrini",
      "out|headset-outline|Öncelikli destek"
    ]'::jsonb,
    25000,
    'TRY',
    30,
    false,
    false,
    10,
    false,
    true,
    10,
    1,
    TIMESTAMPTZ '2020-01-01 00:00:00+00',
    TIMESTAMPTZ '2020-01-01 00:00:00+00'
),
(
    'a0000000-0000-4000-8000-000000000012',
    'PREMIUM',
    'Premium',
    'Daha fazla görünürlük',
    'Önerilen',
    '[
      "in|time-outline|45 gün yayın",
      "in|images-outline|5 görsel + kapak",
      "in|flash-outline|Acil ilan rozeti",
      "in|star-outline|Öne çıkan ilan · 7 gün",
      "in|share-social-outline|Sosyal medya · 1 platform",
      "out|trophy-outline|Anasayfa vitrini",
      "out|headset-outline|Öncelikli destek"
    ]'::jsonb,
    65000,
    'TRY',
    45,
    true,
    false,
    50,
    false,
    true,
    20,
    1,
    TIMESTAMPTZ '2020-01-01 00:00:00+00',
    TIMESTAMPTZ '2020-01-01 00:00:00+00'
),
(
    'a0000000-0000-4000-8000-000000000013',
    'ULTIMATE',
    'Ultimate',
    'Maksimum erişim',
    NULL,
    '[
      "in|time-outline|60 gün yayın",
      "in|images-outline|5 görsel + kapak",
      "in|flash-outline|Acil ilan rozeti",
      "in|star-outline|Öne çıkan ilan · 30 gün",
      "in|share-social-outline|Instagram, Facebook, X",
      "in|trophy-outline|Anasayfa vitrini",
      "in|headset-outline|Öncelikli destek"
    ]'::jsonb,
    125000,
    'TRY',
    60,
    true,
    true,
    100,
    true,
    true,
    30,
    1,
    TIMESTAMPTZ '2020-01-01 00:00:00+00',
    TIMESTAMPTZ '2020-01-01 00:00:00+00'
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
    search_priority = EXCLUDED.search_priority,
    broadcast_on_publish = EXCLUDED.broadcast_on_publish,
    is_active = true,
    sort_order = EXCLUDED.sort_order,
    updated_at = NOW(),
    version = hrd_packages.version + 1;

-- Hide non-catalog / self-test packages from the public listing.
UPDATE hrd_packages
SET is_active = false,
    updated_at = NOW(),
    version = version + 1
WHERE is_active = true
  AND code NOT IN ('STANDARD', 'PREMIUM', 'ULTIMATE');

-- +goose Down
UPDATE hrd_packages
SET is_active = false,
    updated_at = NOW(),
    version = version + 1
WHERE code IN ('STANDARD', 'PREMIUM', 'ULTIMATE');
