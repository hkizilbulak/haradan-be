-- +goose Up
CREATE TABLE hrd_stud_farms (
    id uuid NOT NULL,
    first_name varchar(100) NOT NULL,
    last_name varchar(100) NOT NULL,
    email varchar(320) NOT NULL,
    phone varchar(32) NULL,
    location varchar(255) NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT hrd_stud_farms_pkey PRIMARY KEY (id)
);

CREATE TABLE hrd_stud_farm_notes (
    id uuid NOT NULL,
    stud_farm_id uuid NOT NULL,
    interviewer_name varchar(100) NOT NULL,
    interview_date timestamptz NOT NULL,
    notes_url text NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT hrd_stud_farm_notes_pkey PRIMARY KEY (id),
    CONSTRAINT hrd_stud_farm_notes_stud_farm_id_fkey FOREIGN KEY (stud_farm_id) REFERENCES hrd_stud_farms(id) ON DELETE CASCADE
);

CREATE INDEX hrd_stud_farm_notes_stud_farm_id_idx ON hrd_stud_farm_notes(stud_farm_id);

-- +goose Down
DROP TABLE hrd_stud_farm_notes;
DROP TABLE hrd_stud_farms;
