-- +goose Up
CREATE TYPE hrd_agreement_type AS ENUM (
    'MEMBERSHIP_AGREEMENT', 
    'KVKK_EXPLICIT_CONSENT', 
    'COMMUNICATION_EMAIL', 
    'COMMUNICATION_SMS', 
    'COMMUNICATION_WHATSAPP', 
    'DISTANCE_SALES_AGREEMENT'
);

CREATE TABLE hrd_user_settings (
    user_id uuid PRIMARY KEY,
    allow_email boolean NOT NULL DEFAULT false,
    allow_sms boolean NOT NULL DEFAULT false,
    allow_whatsapp boolean NOT NULL DEFAULT false,
    CONSTRAINT hrd_user_settings_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrd_users (id) ON DELETE CASCADE
);

CREATE TABLE hrd_user_consent_logs (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    agreement_type hrd_agreement_type NOT NULL,
    version varchar(50) NOT NULL,
    is_granted boolean NOT NULL,
    ip_address varchar(64),
    user_agent varchar(512),
    channel varchar(32) NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT hrd_user_consent_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrd_users (id) ON DELETE CASCADE,
    CONSTRAINT hrd_user_consent_logs_channel_check CHECK (channel IN ('WEB', 'IOS', 'ANDROID', 'WEB_BO', 'CALL_CENTER'))
);

CREATE INDEX hrd_user_consent_logs_lookup_idx ON hrd_user_consent_logs (user_id, agreement_type, created_at DESC);

-- +goose Down
DROP TABLE hrd_user_consent_logs;
DROP TABLE hrd_user_settings;
DROP TYPE hrd_agreement_type;
