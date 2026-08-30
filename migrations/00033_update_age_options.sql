-- +goose Up
-- Update options for HORSE_AGE and STALLION_AGE in hrd_category_properties

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
WHERE code IN ('HORSE_AGE', 'STALLION_AGE');

-- +goose Down
UPDATE hrd_category_properties
SET options = '[
  {"value": "Tay (0-1 Yaş)", "label": "Tay (0-1 Yaş)"},
  {"value": "2 Yaş", "label": "2 Yaş"},
  {"value": "3 Yaş", "label": "3 Yaş"},
  {"value": "4 Yaş", "label": "4 Yaş"},
  {"value": "5+ Yaş", "label": "5+ Yaş"}
]'::jsonb,
version = version + 1,
updated_at = NOW()
WHERE code = 'HORSE_AGE';
