-- +goose Up
-- Refresh only known English seed/placeholder display names.
-- Does not overwrite customized names or mutate timestamps.

UPDATE hrd_notification_templates
SET name = 'Paket ilanı yayınlandı'
WHERE event_type = 'PACKAGE_ADVERT_PUBLISHED'
  AND name IN (
    'Package advert published placeholder',
    'Advanced advert published placeholder'
  );

UPDATE hrd_notification_templates
SET name = 'Paket bitiş hatırlatması (1 gün)'
WHERE event_type = 'PACKAGE_EXPIRY_1_DAY'
  AND name IN (
    'Package expiry 1 day placeholder',
    'Package expiry 3 days placeholder'
  );

UPDATE hrd_notification_templates
SET name = 'Paket bitiş hatırlatması (5 gün)'
WHERE event_type = 'PACKAGE_EXPIRY_5_DAYS'
  AND name IN (
    'Package expiry 5 days placeholder',
    'Package expiry 10 days placeholder'
  );

UPDATE hrd_notification_templates
SET name = 'Acil ilan aktifleştirildi'
WHERE event_type = 'URGENT_ADVERT_ACTIVATED'
  AND name = 'Urgent advert activated placeholder';

-- +goose Down
UPDATE hrd_notification_templates
SET name = 'Package advert published placeholder'
WHERE event_type = 'PACKAGE_ADVERT_PUBLISHED'
  AND name = 'Paket ilanı yayınlandı';

UPDATE hrd_notification_templates
SET name = 'Package expiry 1 day placeholder'
WHERE event_type = 'PACKAGE_EXPIRY_1_DAY'
  AND name = 'Paket bitiş hatırlatması (1 gün)';

UPDATE hrd_notification_templates
SET name = 'Package expiry 5 days placeholder'
WHERE event_type = 'PACKAGE_EXPIRY_5_DAYS'
  AND name = 'Paket bitiş hatırlatması (5 gün)';

UPDATE hrd_notification_templates
SET name = 'Urgent advert activated placeholder'
WHERE event_type = 'URGENT_ADVERT_ACTIVATED'
  AND name = 'Acil ilan aktifleştirildi';
