package advert

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	domainhorse "github.com/hkizilbulak/haradan-be/internal/domain/horse"
)

func TestHorseEnricher_SaleHorse(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	cat := domaincatalog.Category{
		ID:       uuid.New(),
		Slug:     "satilik-yaris-ati",
		Name:     "Satılık Yarış Atı",
		AllowTjk: true,
	}

	breed := "İngiliz"
	gender := "e"
	coat := "d"
	birthYear := 2022 // 4 years old in 2026
	sire := "PERSIAN BOLD"
	dam := "ROSA BLANCHE"
	detailJSON := []byte(`{
		"profile": {
			"maidenSire": "BALIDAR",
			"owner": "Özdemir Atman",
			"grower": "Atman Ekürisi",
			"birthDate": "2022-04-15"
		},
		"heightCm": 165
	}`)

	h := domainhorse.Horse{
		ID:           uuid.New(),
		TJKNumber:    "85948",
		OriginalName: "BOLD PILOT",
		BirthYear:    &birthYear,
		SireName:     &sire,
		DamName:      &dam,
		Breed:        &breed,
		Gender:       &gender,
		Coat:         &coat,
		Detail:       detailJSON,
	}

	enriched, err := EnrichHorseProperties(cat, h, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var props map[string]any
	if err := json.Unmarshal(enriched, &props); err != nil {
		t.Fatalf("unmarshal enriched: %v", err)
	}

	if props["HORSE_BREED"] != "İngiliz (Thoroughbred)" {
		t.Errorf("expected HORSE_BREED 'İngiliz (Thoroughbred)', got %v", props["HORSE_BREED"])
	}
	if props["HORSE_AGE"] != "4" {
		t.Errorf("expected HORSE_AGE '4', got %v", props["HORSE_AGE"])
	}
	if props["HORSE_GENDER"] != "Erkek" {
		t.Errorf("expected HORSE_GENDER 'Erkek', got %v", props["HORSE_GENDER"])
	}
	if props["COAT_COLOR"] != "Doru" {
		t.Errorf("expected COAT_COLOR 'Doru', got %v", props["COAT_COLOR"])
	}
	if props["SIRE"] != "PERSIAN BOLD" {
		t.Errorf("expected SIRE 'PERSIAN BOLD', got %v", props["SIRE"])
	}
	if props["DAM"] != "ROSA BLANCHE" {
		t.Errorf("expected DAM 'ROSA BLANCHE', got %v", props["DAM"])
	}
	if props["DAMSIRE"] != "BALIDAR" {
		t.Errorf("expected DAMSIRE 'BALIDAR', got %v", props["DAMSIRE"])
	}
	if props["REGISTERED_NAME"] != "BOLD PILOT" {
		t.Errorf("expected REGISTERED_NAME 'BOLD PILOT', got %v", props["REGISTERED_NAME"])
	}
	if props["TJK_NUMBER"] != "85948" {
		t.Errorf("expected TJK_NUMBER '85948', got %v", props["TJK_NUMBER"])
	}
	if props["BREEDER"] != "Atman Ekürisi" {
		t.Errorf("expected BREEDER 'Atman Ekürisi', got %v", props["BREEDER"])
	}
	if props["OWNER"] != "Özdemir Atman" {
		t.Errorf("expected OWNER 'Özdemir Atman', got %v", props["OWNER"])
	}
	if props["BIRTH_DATE"] != "2022-04-15" {
		t.Errorf("expected BIRTH_DATE '2022-04-15', got %v", props["BIRTH_DATE"])
	}
	if props["HEIGHT_CM"] != float64(165) {
		t.Errorf("expected HEIGHT_CM 165, got %v", props["HEIGHT_CM"])
	}
}

func TestHorseEnricher_StudService(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	cat := domaincatalog.Category{
		ID:       uuid.New(),
		Slug:     "arap-aygir",
		Name:     "Arap Aygır",
		AllowTjk: true,
	}

	breed := "Safkan Arap"
	gender := "e"
	coat := "k"
	birthYear := 2019 // 7 years old in 2026 -> "5+"
	sire := "ÖZGÜNHAN"
	dam := "KEMİYETÜLIRAK.55"
	detailJSON := []byte(`{
		"profile": {
			"maidenSire": "HAVUÇEROL",
			"grower": "Karacabey Tarım İşletmesi"
		}
	}`)

	h := domainhorse.Horse{
		ID:           uuid.New(),
		TJKNumber:    "44556",
		OriginalName: "RÜZGARIN OĞLU",
		BirthYear:    &birthYear,
		SireName:     &sire,
		DamName:      &dam,
		Breed:        &breed,
		Gender:       &gender,
		Coat:         &coat,
		Detail:       detailJSON,
	}

	enriched, err := EnrichHorseProperties(cat, h, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var props map[string]any
	if err := json.Unmarshal(enriched, &props); err != nil {
		t.Fatalf("unmarshal enriched: %v", err)
	}

	if props["STALLION_BREED"] != "Arap" {
		t.Errorf("expected STALLION_BREED 'Arap', got %v", props["STALLION_BREED"])
	}
	if props["STALLION_AGE"] != "7" {
		t.Errorf("expected STALLION_AGE '7', got %v", props["STALLION_AGE"])
	}
	if props["COAT_COLOR"] != "Kır" {
		t.Errorf("expected COAT_COLOR 'Kır', got %v", props["COAT_COLOR"])
	}
	if props["studHorseName"] != "RÜZGARIN OĞLU" {
		t.Errorf("expected studHorseName 'RÜZGARIN OĞLU', got %v", props["studHorseName"])
	}
	if props["studSire"] != "ÖZGÜNHAN" {
		t.Errorf("expected studSire 'ÖZGÜNHAN', got %v", props["studSire"])
	}
	if props["studDam"] != "KEMİYETÜLIRAK.55" {
		t.Errorf("expected studDam 'KEMİYETÜLIRAK.55', got %v", props["studDam"])
	}
	if props["studDamsire"] != "HAVUÇEROL" {
		t.Errorf("expected studDamsire 'HAVUÇEROL', got %v", props["studDamsire"])
	}
	if props["TJK_NUMBER"] != "44556" {
		t.Errorf("expected TJK_NUMBER '44556', got %v", props["TJK_NUMBER"])
	}
}
