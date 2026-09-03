-- +goose Up
-- Update options for stud properties in hrd_category_properties

UPDATE hrd_category_properties
SET options = '[
  {"value": "0", "label": "0"},
  {"value": "1", "label": "1"},
  {"value": "1.5", "label": "1.5"},
  {"value": "2", "label": "2"},
  {"value": "3", "label": "3"},
  {"value": "4", "label": "4"},
  {"value": "5", "label": "5"},
  {"value": "6", "label": "6"},
  {"value": "7", "label": "7"},
  {"value": "8", "label": "8"},
  {"value": "9", "label": "9"},
  {"value": "10-15 arası", "label": "10-15 arası"},
  {"value": "15 üzeri", "label": "15 üzeri"}
]'::jsonb,
version = version + 1,
updated_at = NOW()
WHERE code IN ('studAge', 'STALLION_AGE', 'HORSE_AGE');

UPDATE hrd_category_properties
SET options = '[
  {"value": "Doru", "label": "Doru"},
  {"value": "Al", "label": "Al"},
  {"value": "Kır", "label": "Kır"},
  {"value": "Beyaz", "label": "Beyaz"},
  {"value": "Yağız", "label": "Yağız"},
  {"value": "Kula", "label": "Kula"},
  {"value": "Boz", "label": "Boz"},
  {"value": "Kestane", "label": "Kestane"}
]'::jsonb,
version = version + 1,
updated_at = NOW()
WHERE code IN ('studCoatColor', 'COAT_COLOR');

UPDATE hrd_category_properties
SET options = '[
  {"value": "Arap", "label": "Arap"},
  {"value": "İngiliz", "label": "İngiliz"}
]'::jsonb,
version = version + 1,
updated_at = NOW()
WHERE code IN ('studBreed', 'STALLION_BREED');

-- +goose Down
UPDATE hrd_category_properties
SET options = '[
  {"value": "3 Yaş", "label": "3 Yaş"},
  {"value": "4 Yaş", "label": "4 Yaş"},
  {"value": "5+ Yaş", "label": "5+ Yaş"}
]'::jsonb,
version = version + 1,
updated_at = NOW()
WHERE code = 'studAge';
