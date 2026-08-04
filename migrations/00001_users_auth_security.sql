-- +goose Up
CREATE TABLE hrd_users (
    id uuid NOT NULL,
    email varchar(320) NOT NULL,
    email_normalized varchar(320) NOT NULL,
    password_hash varchar(255) NOT NULL,
    role varchar(16) NOT NULL DEFAULT 'user',
    status varchar(16) NOT NULL DEFAULT 'ACTIVE',
    email_verified_at timestamptz NULL,
    first_name varchar(100) NOT NULL,
    last_name varchar(100) NOT NULL,
    phone varchar(32) NULL,
    security_stamp uuid NOT NULL,
    failed_login_count integer NOT NULL DEFAULT 0,
    locked_until timestamptz NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_users_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_users_email_normalized_key UNIQUE (email_normalized),
    CONSTRAINT hrd_users_role_check CHECK (role IN ('user', 'admin')),
    CONSTRAINT hrd_users_status_check CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT hrd_users_failed_login_count_check CHECK (failed_login_count >= 0)
);

CREATE INDEX hrd_users_status_idx ON hrd_users (status);

CREATE TABLE hrd_auth_sessions (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    client_context varchar(16) NOT NULL,
    refresh_token_hash varchar(255) NOT NULL,
    family_id uuid NOT NULL,
    replaced_by_session_id uuid NULL,
    absolute_expires_at timestamptz NOT NULL,
    idle_expires_at timestamptz NOT NULL,
    revoked_at timestamptz NULL,
    revoke_reason varchar(64) NULL,
    created_at timestamptz NOT NULL,
    last_used_at timestamptz NOT NULL,
    user_agent varchar(512) NULL,
    ip_hash varchar(128) NULL,
    CONSTRAINT hrd_auth_sessions_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_auth_sessions_user_id_fkey FOREIGN KEY (user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_auth_sessions_replaced_by_session_id_fkey FOREIGN KEY (replaced_by_session_id)
        REFERENCES hrd_auth_sessions (id) ON DELETE SET NULL,
    CONSTRAINT hrd_auth_sessions_refresh_token_hash_key UNIQUE (refresh_token_hash),
    CONSTRAINT hrd_auth_sessions_client_context_check
        CHECK (client_context IN ('PUBLIC_WEB', 'MOBILE', 'ADMIN_BO')),
    CONSTRAINT hrd_auth_sessions_idle_le_absolute_check
        CHECK (idle_expires_at <= absolute_expires_at),
    CONSTRAINT hrd_auth_sessions_created_le_last_used_check
        CHECK (created_at <= last_used_at),
    CONSTRAINT hrd_auth_sessions_revoke_reason_requires_revoked_at_check
        CHECK (revoke_reason IS NULL OR revoked_at IS NOT NULL),
    CONSTRAINT hrd_auth_sessions_no_self_replace_check
        CHECK (replaced_by_session_id IS NULL OR replaced_by_session_id <> id)
);

CREATE INDEX hrd_auth_sessions_user_id_idx ON hrd_auth_sessions (user_id);
CREATE INDEX hrd_auth_sessions_family_id_idx ON hrd_auth_sessions (family_id);
CREATE INDEX hrd_auth_sessions_active_lookup_idx ON hrd_auth_sessions (user_id, client_context)
    WHERE revoked_at IS NULL;

CREATE TABLE hrd_one_time_credentials (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    purpose varchar(32) NOT NULL,
    token_hash varchar(255) NOT NULL,
    target_email varchar(320) NULL,
    target_email_normalized varchar(320) NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz NULL,
    invalidated_at timestamptz NULL,
    created_at timestamptz NOT NULL,
    request_ip_hash varchar(128) NULL,
    CONSTRAINT hrd_one_time_credentials_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_one_time_credentials_user_id_fkey FOREIGN KEY (user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_one_time_credentials_token_hash_key UNIQUE (token_hash),
    CONSTRAINT hrd_one_time_credentials_purpose_check
        CHECK (purpose IN ('EMAIL_VERIFICATION', 'EMAIL_CHANGE_VERIFICATION', 'PASSWORD_RESET')),
    CONSTRAINT hrd_one_time_credentials_target_email_by_purpose_check CHECK (
        (
            purpose IN ('EMAIL_VERIFICATION', 'EMAIL_CHANGE_VERIFICATION')
            AND target_email IS NOT NULL
            AND target_email_normalized IS NOT NULL
        )
        OR (
            purpose = 'PASSWORD_RESET'
            AND target_email IS NULL
            AND target_email_normalized IS NULL
        )
    ),
    CONSTRAINT hrd_one_time_credentials_consumed_xor_invalidated_check
        CHECK (NOT (consumed_at IS NOT NULL AND invalidated_at IS NOT NULL)),
    CONSTRAINT hrd_one_time_credentials_expires_after_created_check
        CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX hrd_one_time_credentials_one_active_per_user_purpose_key
    ON hrd_one_time_credentials (user_id, purpose)
    WHERE consumed_at IS NULL AND invalidated_at IS NULL;

CREATE TABLE hrd_security_events (
    id uuid NOT NULL,
    subject_user_id uuid NULL,
    actor_user_id uuid NULL,
    event_type varchar(64) NOT NULL,
    client_context varchar(16) NULL,
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL,
    CONSTRAINT hrd_security_events_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_security_events_subject_user_id_fkey FOREIGN KEY (subject_user_id)
        REFERENCES hrd_users (id) ON DELETE SET NULL,
    CONSTRAINT hrd_security_events_actor_user_id_fkey FOREIGN KEY (actor_user_id)
        REFERENCES hrd_users (id) ON DELETE SET NULL,
    CONSTRAINT hrd_security_events_event_type_check CHECK (event_type IN (
        'LOGIN_SUCCESS',
        'LOGIN_FAILURE',
        'LOGOUT',
        'SESSION_REVOKED',
        'ALL_SESSIONS_REVOKED',
        'REFRESH_REPLAY_DETECTED',
        'PASSWORD_CHANGE',
        'PASSWORD_RESET',
        'EMAIL_VERIFICATION',
        'EMAIL_CHANGE',
        'ROLE_CHANGE',
        'ACCOUNT_STATUS_CHANGE',
        'BO_CONTEXT_REJECTED'
    )),
    CONSTRAINT hrd_security_events_client_context_check
        CHECK (client_context IS NULL OR client_context IN ('PUBLIC_WEB', 'MOBILE', 'ADMIN_BO')),
    CONSTRAINT hrd_security_events_metadata_object_check
        CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX hrd_security_events_subject_created_idx
    ON hrd_security_events (subject_user_id, created_at DESC);
CREATE INDEX hrd_security_events_actor_created_idx
    ON hrd_security_events (actor_user_id, created_at DESC);
CREATE INDEX hrd_security_events_type_created_idx
    ON hrd_security_events (event_type, created_at DESC);

-- +goose Down
DROP TABLE hrd_security_events;
DROP TABLE hrd_one_time_credentials;
DROP TABLE hrd_auth_sessions;
DROP TABLE hrd_users;
