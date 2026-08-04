-- +goose Up
CREATE TABLE hrd_horses (
    id uuid NOT NULL,
    tjk_number varchar(64) NOT NULL,
    original_name varchar(200) NOT NULL,
    name_normalized varchar(200) NOT NULL,
    birth_year smallint NULL,
    sire_name varchar(200) NULL,
    dam_name varchar(200) NULL,
    breed varchar(120) NULL,
    gender varchar(32) NULL,
    coat varchar(64) NULL,
    detail jsonb NOT NULL DEFAULT '{}',
    last_synced_at timestamptz NULL,
    last_seen_at timestamptz NULL,
    source_updated_at timestamptz NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_horses_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_horses_tjk_number_key UNIQUE (tjk_number),
    CONSTRAINT hrd_horses_birth_year_check
        CHECK (birth_year IS NULL OR (birth_year >= 1800 AND birth_year <= 2200)),
    CONSTRAINT hrd_horses_detail_object_check CHECK (jsonb_typeof(detail) = 'object')
);

CREATE INDEX hrd_horses_name_normalized_prefix_idx
    ON hrd_horses (name_normalized varchar_pattern_ops);

-- +goose Down
DROP TABLE hrd_horses;
