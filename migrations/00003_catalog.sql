-- +goose Up
CREATE TABLE hrd_categories (
    id uuid NOT NULL,
    parent_id uuid NULL,
    slug varchar(120) NOT NULL,
    name varchar(160) NOT NULL,
    description text NULL,
    is_active boolean NOT NULL DEFAULT true,
    sort_order integer NOT NULL DEFAULT 0,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_categories_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_categories_parent_id_fkey FOREIGN KEY (parent_id)
        REFERENCES hrd_categories (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_categories_slug_key UNIQUE (slug),
    CONSTRAINT hrd_categories_no_self_parent_check CHECK (parent_id IS NULL OR parent_id <> id),
    CONSTRAINT hrd_categories_sort_order_nonnegative_check CHECK (sort_order >= 0),
    CONSTRAINT hrd_categories_version_positive_check CHECK (version > 0)
);

CREATE INDEX hrd_categories_parent_id_sort_idx ON hrd_categories (parent_id, sort_order);

CREATE TABLE hrd_category_properties (
    id uuid NOT NULL,
    category_id uuid NOT NULL,
    code varchar(64) NOT NULL,
    title varchar(160) NOT NULL,
    help_text text NULL,
    data_type varchar(32) NOT NULL,
    is_required boolean NOT NULL DEFAULT false,
    is_public_visible boolean NOT NULL DEFAULT true,
    is_form_visible boolean NOT NULL DEFAULT true,
    is_filterable boolean NOT NULL DEFAULT false,
    sort_order integer NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true,
    options jsonb NOT NULL DEFAULT '[]',
    validation jsonb NOT NULL DEFAULT '{}',
    default_value jsonb NULL,
    ui_metadata jsonb NOT NULL DEFAULT '{}',
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_category_properties_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_category_properties_category_id_fkey FOREIGN KEY (category_id)
        REFERENCES hrd_categories (id) ON DELETE RESTRICT,
    CONSTRAINT hrd_category_properties_category_id_code_key UNIQUE (category_id, code),
    CONSTRAINT hrd_category_properties_data_type_check CHECK (data_type IN (
        'STRING', 'TEXT', 'INTEGER', 'DECIMAL', 'BOOLEAN', 'SINGLE_SELECT', 'YEAR'
    )),
    CONSTRAINT hrd_category_properties_sort_order_nonnegative_check CHECK (sort_order >= 0),
    CONSTRAINT hrd_category_properties_options_array_check
        CHECK (jsonb_typeof(options) = 'array'),
    CONSTRAINT hrd_category_properties_validation_object_check
        CHECK (jsonb_typeof(validation) = 'object'),
    CONSTRAINT hrd_category_properties_ui_metadata_object_check
        CHECK (jsonb_typeof(ui_metadata) = 'object'),
    CONSTRAINT hrd_category_properties_version_positive_check CHECK (version > 0)
);

CREATE INDEX hrd_category_properties_category_active_sort_idx
    ON hrd_category_properties (category_id, is_active, sort_order);

-- +goose Down
DROP TABLE hrd_category_properties;
DROP TABLE hrd_categories;
