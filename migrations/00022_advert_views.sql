-- +goose Up
ALTER TABLE hrd_adverts ADD COLUMN view_count integer NOT NULL DEFAULT 0;

CREATE TABLE hrd_advert_views (
    advert_id uuid NOT NULL,
    ip_address varchar(45) NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT hrd_advert_views_pkey PRIMARY KEY (advert_id, ip_address),
    CONSTRAINT hrd_advert_views_advert_id_fkey FOREIGN KEY (advert_id)
        REFERENCES hrd_adverts (id) ON DELETE CASCADE
);

CREATE INDEX hrd_advert_views_advert_id_idx ON hrd_advert_views (advert_id);

-- +goose Down
DROP TABLE IF EXISTS hrd_advert_views;
ALTER TABLE hrd_adverts DROP COLUMN IF EXISTS view_count;
