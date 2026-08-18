-- +goose Up
ALTER TABLE hrd_banners
    DROP CONSTRAINT hrd_banners_placement_check;

ALTER TABLE hrd_banners
    ADD CONSTRAINT hrd_banners_placement_check
        CHECK (placement IN ('HOMEPAGE', 'HOMEPAGE_HERO', 'HOMEPAGE_PROMO', 'LISTING_DETAIL', 'SEARCH'));

-- +goose Down
ALTER TABLE hrd_banners
    DROP CONSTRAINT hrd_banners_placement_check;

ALTER TABLE hrd_banners
    ADD CONSTRAINT hrd_banners_placement_check
        CHECK (placement IN ('HOMEPAGE', 'LISTING_DETAIL', 'SEARCH'));
