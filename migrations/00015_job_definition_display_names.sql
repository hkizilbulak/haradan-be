-- +goose Up
-- Refresh only known English seed job display names/descriptions.
-- Does not overwrite customized names or mutate timestamps / cron / active flags.

UPDATE hrd_job_definitions
SET name = 'TJK at özeti senkronu',
    description = 'TJK toplu özet senkronu (PageNumber 0-tabanlı).'
WHERE job_key = 'TJK_SYNC'
  AND name = 'TJK horse summary sync';

UPDATE hrd_job_definitions
SET name = 'Paket bitiş hatırlatma taraması',
    description = 'Günlük 5 günlük ve 1 günlük paket bitiş hatırlatmaları ile süre sonu taraması.'
WHERE job_key = 'PACKAGE_EXPIRY_SCAN'
  AND name = 'Package expiry reminder scan';

UPDATE hrd_job_definitions
SET name = 'Medya depolama eşitleme',
    description = 'Takılı veya yetim medya nesnelerini özel B2 depolama ile eşitle.'
WHERE job_key = 'MEDIA_RECONCILE'
  AND name = 'Media storage reconcile';

-- +goose Down
UPDATE hrd_job_definitions
SET name = 'TJK horse summary sync',
    description = 'Legacy-aligned TJK bulk summary sync (PageNumber 0-based).'
WHERE job_key = 'TJK_SYNC'
  AND name = 'TJK at özeti senkronu';

UPDATE hrd_job_definitions
SET name = 'Package expiry reminder scan',
    description = 'Daily scan for 5-day and 1-day package expiry reminders and expiry.'
WHERE job_key = 'PACKAGE_EXPIRY_SCAN'
  AND name = 'Paket bitiş hatırlatma taraması';

UPDATE hrd_job_definitions
SET name = 'Media storage reconcile',
    description = 'Reconcile orphaned or stuck media objects against private B2 storage.'
WHERE job_key = 'MEDIA_RECONCILE'
  AND name = 'Medya depolama eşitleme';
