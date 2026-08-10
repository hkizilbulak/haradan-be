-- +goose Up
-- Recipient-scoped email delivery state for notification fan-out and expiry reminders.
ALTER TABLE hrd_user_notification_states
    ADD COLUMN email_status varchar(32) NOT NULL DEFAULT 'NOT_REQUESTED',
    ADD COLUMN email_idempotency_key varchar(256) NULL,
    ADD COLUMN email_attempt_count integer NOT NULL DEFAULT 0,
    ADD COLUMN email_last_attempt_at timestamptz NULL,
    ADD COLUMN email_sent_at timestamptz NULL,
    ADD COLUMN email_last_error varchar(512) NULL;

ALTER TABLE hrd_user_notification_states
    ADD CONSTRAINT hrd_user_notification_states_email_status_check
        CHECK (email_status IN ('NOT_REQUESTED', 'QUEUED', 'SENT', 'FAILED')),
    ADD CONSTRAINT hrd_user_notification_states_email_attempt_nonnegative_check
        CHECK (email_attempt_count >= 0),
    ADD CONSTRAINT hrd_user_notification_states_email_idempotency_key_len_check
        CHECK (
            email_idempotency_key IS NULL
            OR char_length(email_idempotency_key) BETWEEN 1 AND 256
        ),
    ADD CONSTRAINT hrd_user_notification_states_email_idempotency_required_check
        CHECK (
            email_status = 'NOT_REQUESTED'
            OR email_idempotency_key IS NOT NULL
        ),
    ADD CONSTRAINT hrd_user_notification_states_email_sent_at_check
        CHECK (
            (
                email_status = 'SENT'
                AND email_sent_at IS NOT NULL
            )
            OR (
                email_status = 'NOT_REQUESTED'
                AND email_sent_at IS NULL
            )
            OR (
                email_status IN ('QUEUED', 'FAILED')
            )
        ),
    ADD CONSTRAINT hrd_user_notification_states_email_last_error_len_check
        CHECK (
            email_last_error IS NULL
            OR (
                char_length(email_last_error) BETWEEN 1 AND 512
                AND octet_length(convert_to(email_last_error, 'UTF8')) <= 512
            )
        );

CREATE UNIQUE INDEX hrd_user_notification_states_email_idempotency_key_uidx
    ON hrd_user_notification_states (email_idempotency_key)
    WHERE email_idempotency_key IS NOT NULL;

CREATE INDEX hrd_user_notification_states_notification_email_user_idx
    ON hrd_user_notification_states (notification_id, email_status, user_id);

CREATE INDEX hrd_user_notification_states_user_email_status_idx
    ON hrd_user_notification_states (user_id, email_status);

-- Fan-out eligibility helpers (ACTIVE / ACTIVE+verified).
CREATE INDEX hrd_users_active_id_idx
    ON hrd_users (id)
    WHERE status = 'ACTIVE';

CREATE INDEX hrd_users_active_verified_id_idx
    ON hrd_users (id)
    WHERE status = 'ACTIVE' AND email_verified_at IS NOT NULL;

-- +goose Down
DROP INDEX hrd_users_active_verified_id_idx;
DROP INDEX hrd_users_active_id_idx;
DROP INDEX hrd_user_notification_states_user_email_status_idx;
DROP INDEX hrd_user_notification_states_notification_email_user_idx;
DROP INDEX hrd_user_notification_states_email_idempotency_key_uidx;

ALTER TABLE hrd_user_notification_states
    DROP CONSTRAINT hrd_user_notification_states_email_last_error_len_check,
    DROP CONSTRAINT hrd_user_notification_states_email_sent_at_check,
    DROP CONSTRAINT hrd_user_notification_states_email_idempotency_required_check,
    DROP CONSTRAINT hrd_user_notification_states_email_idempotency_key_len_check,
    DROP CONSTRAINT hrd_user_notification_states_email_attempt_nonnegative_check,
    DROP CONSTRAINT hrd_user_notification_states_email_status_check;

ALTER TABLE hrd_user_notification_states
    DROP COLUMN email_last_error,
    DROP COLUMN email_sent_at,
    DROP COLUMN email_last_attempt_at,
    DROP COLUMN email_attempt_count,
    DROP COLUMN email_idempotency_key,
    DROP COLUMN email_status;
