-- +goose Up
CREATE TABLE hrd_communication_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL, -- e.g., 'INTRO', 'MEMBERSHIP' or custom 'Haradan.com Tanıtımı ve İlan Daveti'
    channel VARCHAR(50) NOT NULL, -- 'PHONE', 'WHATSAPP', 'SOCIAL_MEDIA', 'EMAIL'
    content TEXT NOT NULL,
    subject VARCHAR(255), -- for EMAIL
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Basic Seed for default templates
INSERT INTO hrd_communication_templates (title, channel, content, is_default)
VALUES 
    ('INTRO', 'PHONE', 'Merhabalar {person_name}, Haradan.com platformundan arıyorum. {stud_name} bünyesindeki değerli safkanlarınızı ve güncel satış ilanlarınızı takip ediyoruz.\n\n[Görüşme Notları]\n- Platformumuzun Türkiye''nin yeni nesil at pazaryeri olduğunu belirtin.\n- Haraya özel vitrin özelliklerinden bahsedin.\n- İtiraz gelirse: "Ücretsiz profil oluşturarak test edebilirsiniz" seçeneğini sunun.', true),
    ('INTRO', 'WHATSAPP', 'Merhabalar {person_name}, Haradan.com ekibinden ulaşıyorum. {stud_name} için hazırladığımız dijital ilan ve haralara özel vitrin avantajlarımız hakkında bilgi vermek isteriz. Detaylı bilgi için ne zaman görüşebiliriz?', true);

-- +goose Down
DROP TABLE IF EXISTS hrd_communication_templates;
