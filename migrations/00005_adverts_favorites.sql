-- +goose Up
CREATE TABLE hrd_adverts (
    id uuid NOT NULL,
    owner_user_id uuid NOT NULL,
    category_id uuid NULL,
    district_id uuid NULL,
    horse_id uuid NULL,
    title varchar(200) NULL,
    description text NULL,
    price_amount_minor bigint NULL,
    price_currency varchar(3) NULL,
    status varchar(32) NOT NULL DEFAULT 'DRAFT',
    properties jsonb NOT NULL DEFAULT '{}',
    published_at timestamptz NULL,
    version integer NOT NULL DEFAULT 1,
    media_version integer NOT NULL DEFAULT 1,
    deleted_at timestamptz NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_adverts_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_adverts_owner_user_id_fkey FOREIGN KEY (owner_user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_adverts_category_id_fkey FOREIGN KEY (category_id)
        REFERENCES hrd_categories (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_adverts_district_id_fkey FOREIGN KEY (district_id)
        REFERENCES hrd_districts (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_adverts_status_check CHECK (status IN (
        'DRAFT', 'PENDING_REVIEW', 'CHANGES_REQUESTED', 'PUBLISHED',
        'REJECTED', 'SUSPENDED', 'SOLD', 'ARCHIVED'
    )),
    CONSTRAINT hrd_adverts_price_pair_check CHECK (
        (price_amount_minor IS NULL AND price_currency IS NULL)
        OR (price_amount_minor IS NOT NULL AND price_currency IS NOT NULL)
    ),
    CONSTRAINT hrd_adverts_price_amount_minor_check
        CHECK (price_amount_minor IS NULL OR price_amount_minor >= 0),
    CONSTRAINT hrd_adverts_price_currency_format_check
        CHECK (price_currency IS NULL OR price_currency ~ '^[A-Z]{3}$'),
    CONSTRAINT hrd_adverts_properties_object_check
        CHECK (jsonb_typeof(properties) = 'object'),
    CONSTRAINT hrd_adverts_published_at_when_published_check
        CHECK (status <> 'PUBLISHED' OR published_at IS NOT NULL),
    CONSTRAINT hrd_adverts_deleted_at_draft_only_check
        CHECK (deleted_at IS NULL OR status = 'DRAFT'),
    CONSTRAINT hrd_adverts_version_positive_check CHECK (version > 0),
    CONSTRAINT hrd_adverts_media_version_positive_check CHECK (media_version > 0),
    CONSTRAINT hrd_adverts_reviewed_status_required_fields_check CHECK (
        status NOT IN (
            'PENDING_REVIEW', 'PUBLISHED', 'REJECTED', 'SUSPENDED', 'SOLD', 'ARCHIVED'
        )
        OR (
            category_id IS NOT NULL
            AND district_id IS NOT NULL
            AND title IS NOT NULL
            AND description IS NOT NULL
            AND btrim(title) <> ''
            AND btrim(description) <> ''
        )
    )
);

CREATE INDEX hrd_adverts_owner_user_id_created_idx
    ON hrd_adverts (owner_user_id, created_at DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX hrd_adverts_public_newest_idx
    ON hrd_adverts (published_at DESC, id DESC)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
CREATE INDEX hrd_adverts_public_category_newest_idx
    ON hrd_adverts (category_id, published_at DESC, id DESC)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
CREATE INDEX hrd_adverts_public_district_newest_idx
    ON hrd_adverts (district_id, published_at DESC, id DESC)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
CREATE INDEX hrd_adverts_public_horse_newest_idx
    ON hrd_adverts (horse_id, published_at DESC, id DESC)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL AND horse_id IS NOT NULL;

CREATE TABLE hrd_advert_status_history (
    id uuid NOT NULL,
    advert_id uuid NOT NULL,
    from_status varchar(32) NULL,
    to_status varchar(32) NOT NULL,
    actor_user_id uuid NULL,
    is_system boolean NOT NULL DEFAULT false,
    reason text NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT hrd_advert_status_history_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_advert_status_history_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_advert_status_history_actor_user_id_fkey FOREIGN KEY (actor_user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_advert_status_history_from_status_check CHECK (
        from_status IS NULL OR from_status IN (
            'DRAFT', 'PENDING_REVIEW', 'CHANGES_REQUESTED', 'PUBLISHED',
            'REJECTED', 'SUSPENDED', 'SOLD', 'ARCHIVED'
        )
    ),
    CONSTRAINT hrd_advert_status_history_to_status_check CHECK (to_status IN (
        'DRAFT', 'PENDING_REVIEW', 'CHANGES_REQUESTED', 'PUBLISHED',
        'REJECTED', 'SUSPENDED', 'SOLD', 'ARCHIVED'
    )),
    CONSTRAINT hrd_advert_status_history_from_to_check
        CHECK (from_status IS NULL OR from_status <> to_status),
    CONSTRAINT hrd_advert_status_history_actor_system_check CHECK (
        (is_system = true AND actor_user_id IS NULL)
        OR (is_system = false AND actor_user_id IS NOT NULL)
    )
);

CREATE INDEX hrd_advert_status_history_advert_created_idx
    ON hrd_advert_status_history (advert_id, created_at);

CREATE TABLE hrd_favorites (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    advert_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT hrd_favorites_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_favorites_user_id_fkey FOREIGN KEY (user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_favorites_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_favorites_user_id_advert_id_key UNIQUE (user_id, advert_id)
);

CREATE INDEX hrd_favorites_user_created_idx ON hrd_favorites (user_id, created_at DESC);
CREATE INDEX hrd_favorites_advert_id_idx ON hrd_favorites (advert_id);

-- +goose Down
DROP TABLE hrd_favorites;
DROP TABLE hrd_advert_status_history;
DROP TABLE hrd_adverts;
