-- +goose Up
ALTER TABLE hrd_advert_comments
    ADD COLUMN rating smallint NULL;

ALTER TABLE hrd_advert_comments
    ALTER COLUMN content DROP NOT NULL;

ALTER TABLE hrd_advert_comments
    DROP CONSTRAINT hrd_advert_comments_content_not_empty;

ALTER TABLE hrd_advert_comments
    ADD CONSTRAINT hrd_advert_comments_content_or_rating_check CHECK (
        (content IS NOT NULL AND btrim(content) <> '') OR rating IS NOT NULL
    );

-- +goose Down
ALTER TABLE hrd_advert_comments
    DROP CONSTRAINT hrd_advert_comments_content_or_rating_check;

ALTER TABLE hrd_advert_comments
    ALTER COLUMN content SET NOT NULL;

ALTER TABLE hrd_advert_comments
    ADD CONSTRAINT hrd_advert_comments_content_not_empty CHECK (btrim(content) <> '');

ALTER TABLE hrd_advert_comments
    DROP COLUMN rating;
