-- +goose Up
CREATE TABLE hrd_provinces (
    id uuid NOT NULL,
    name varchar(120) NOT NULL,
    name_normalized varchar(120) NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_provinces_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_provinces_name_key UNIQUE (name),
    CONSTRAINT hrd_provinces_name_normalized_key UNIQUE (name_normalized),
    CONSTRAINT hrd_provinces_sort_order_nonnegative_check CHECK (sort_order >= 0)
);

CREATE INDEX hrd_provinces_name_normalized_prefix_idx
    ON hrd_provinces (name_normalized varchar_pattern_ops);

CREATE TABLE hrd_districts (
    id uuid NOT NULL,
    province_id uuid NOT NULL,
    name varchar(120) NOT NULL,
    name_normalized varchar(120) NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_districts_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_districts_province_id_fkey FOREIGN KEY (province_id)
        REFERENCES hrd_provinces (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_districts_province_id_name_key UNIQUE (province_id, name),
    CONSTRAINT hrd_districts_province_id_name_normalized_key UNIQUE (province_id, name_normalized),
    CONSTRAINT hrd_districts_sort_order_nonnegative_check CHECK (sort_order >= 0)
);

CREATE INDEX hrd_districts_province_active_sort_idx
    ON hrd_districts (province_id, is_active, sort_order);
CREATE INDEX hrd_districts_name_normalized_prefix_idx
    ON hrd_districts (name_normalized varchar_pattern_ops);

-- +goose Down
DROP TABLE hrd_districts;
DROP TABLE hrd_provinces;
