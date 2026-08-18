-- +goose Up
CREATE TABLE hrd_advert_comments (
    id uuid NOT NULL,
    advert_id uuid NOT NULL,
    user_id uuid NOT NULL,
    content text NOT NULL,
    status varchar(32) NOT NULL DEFAULT 'PUBLISHED',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    deleted_at timestamptz NULL,
    CONSTRAINT hrd_advert_comments_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_advert_comments_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE CASCADE,
    CONSTRAINT hrd_advert_comments_user_id_fkey FOREIGN KEY (user_id)
        REFERENCES hrd_users (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_advert_comments_status_check CHECK (status IN ('PENDING', 'PUBLISHED', 'REJECTED')),
    CONSTRAINT hrd_advert_comments_content_not_empty CHECK (btrim(content) <> '')
);

CREATE INDEX hrd_idx_advert_comments_lookup
    ON hrd_advert_comments (advert_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL AND status = 'PUBLISHED';

-- +goose Down
DROP TABLE IF EXISTS hrd_advert_comments;
