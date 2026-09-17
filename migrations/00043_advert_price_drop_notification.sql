-- +goose Up

ALTER TABLE hrd_notification_templates
    DROP CONSTRAINT hrd_notification_templates_event_type_check;

ALTER TABLE hrd_notifications
    DROP CONSTRAINT hrd_notifications_event_type_check;

ALTER TABLE hrd_notification_templates
    ADD CONSTRAINT hrd_notification_templates_event_type_check CHECK (event_type IN (
        'PACKAGE_ADVERT_PUBLISHED',
        'URGENT_ADVERT_ACTIVATED',
        'PACKAGE_EXPIRY_5_DAYS',
        'PACKAGE_EXPIRY_1_DAY',
        'ADVERT_PRICE_DROP'
    ));

ALTER TABLE hrd_notifications
    ADD CONSTRAINT hrd_notifications_event_type_check CHECK (event_type IN (
        'PACKAGE_ADVERT_PUBLISHED',
        'URGENT_ADVERT_ACTIVATED',
        'PACKAGE_EXPIRY_5_DAYS',
        'PACKAGE_EXPIRY_1_DAY',
        'ADVERT_PRICE_DROP'
    ));

INSERT INTO hrd_notification_templates (
    id, event_type, name,
    in_app_title_template, in_app_body_template,
    is_active, version, created_at, updated_at
) VALUES (
    gen_random_uuid(), 'ADVERT_PRICE_DROP', 'İlan Fiyat Düşüşü',
    'Favori İlanınızın Fiyatı Düştü!', 'Favorilerinize eklediğiniz "{{.advertTitle}}" başlıklı ilanın fiyatı düştü. İlanı hemen inceleyebilirsiniz.',
    true, 1, now(), now()
);

-- +goose Down

DELETE FROM hrd_notification_templates WHERE event_type = 'ADVERT_PRICE_DROP';

ALTER TABLE hrd_notification_templates
    DROP CONSTRAINT hrd_notification_templates_event_type_check;

ALTER TABLE hrd_notifications
    DROP CONSTRAINT hrd_notifications_event_type_check;

ALTER TABLE hrd_notification_templates
    ADD CONSTRAINT hrd_notification_templates_event_type_check CHECK (event_type IN (
        'PACKAGE_ADVERT_PUBLISHED',
        'URGENT_ADVERT_ACTIVATED',
        'PACKAGE_EXPIRY_5_DAYS',
        'PACKAGE_EXPIRY_1_DAY'
    ));

ALTER TABLE hrd_notifications
    ADD CONSTRAINT hrd_notifications_event_type_check CHECK (event_type IN (
        'PACKAGE_ADVERT_PUBLISHED',
        'URGENT_ADVERT_ACTIVATED',
        'PACKAGE_EXPIRY_5_DAYS',
        'PACKAGE_EXPIRY_1_DAY'
    ));
