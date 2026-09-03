package advert

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	domainhorse "github.com/hkizilbulak/haradan-be/internal/domain/horse"
)

// IsStudCategory determines whether a category represents stud services.
func IsStudCategory(slug string) bool {
	s := strings.ToLower(strings.TrimSpace(slug))
	return s == "asim-hizmetleri" || s == "arap-aygir" || s == "ingiliz-aygir"
}

// NormalizeHorseGender converts raw TJK gender code or text to canonical values.
func NormalizeHorseGender(raw string) string {
	g := strings.ToLower(strings.TrimSpace(raw))
	g = strings.ReplaceAll(g, "i", "i")
	g = strings.ReplaceAll(g, "ı", "i")
	switch {
	case strings.HasPrefix(g, "e"):
		return "Erkek"
	case strings.HasPrefix(g, "d"):
		return "Dişi"
	case strings.HasPrefix(g, "i") || strings.HasPrefix(g, "ı"):
		return "İğdiş"
	default:
		return strings.TrimSpace(raw)
	}
}

// NormalizeHorseBreed converts raw TJK breed to canonical select option value.
func NormalizeHorseBreed(raw string, isStud bool, categorySlug string) string {
	b := strings.ToLower(strings.TrimSpace(raw))
	b = strings.ReplaceAll(b, "i", "i")
	b = strings.ReplaceAll(b, "ı", "i")
	switch {
	case strings.Contains(b, "ingiliz") || strings.Contains(b, "thoroughbred") || categorySlug == "ingiliz-aygir":
		if isStud {
			return "İngiliz"
		}
		return "İngiliz (Thoroughbred)"
	case strings.Contains(b, "arap") || strings.Contains(b, "arabian") || categorySlug == "arap-aygir":
		if isStud {
			return "Arap"
		}
		return "Safkan Arap"
	default:
		return strings.TrimSpace(raw)
	}
}

// NormalizeHorseCoat converts raw TJK coat to canonical option value.
func NormalizeHorseCoat(raw string) string {
	c := strings.ToLower(strings.TrimSpace(raw))
	c = strings.ReplaceAll(c, "i", "i")
	c = strings.ReplaceAll(c, "ı", "i")
	switch c {
	case "d", "doru", "bay":
		return "Doru"
	case "a", "al", "chestnut":
		return "Al"
	case "k", "kır", "kir", "grey", "gray":
		return "Kır"
	case "y", "yağız", "yagiz", "black":
		return "Yağız"
	case "b", "beyaz", "white":
		return "Beyaz"
	case "ku", "kula", "dun", "buckskin":
		return "Kula"
	case "boz", "roan":
		return "Boz"
	case "kestane":
		return "Kestane"
	default:
		if len(raw) > 0 {
			// Title case fallback
			return strings.ToUpper(raw[:1]) + strings.ToLower(raw[1:])
		}
		return raw
	}
}

// NormalizeHorseAge converts birth year to canonical age string.
func NormalizeHorseAge(birthYear int, now time.Time, isStud bool) string {
	age := now.Year() - birthYear
	if age < 0 {
		age = 0
	}
	switch {
	case age == 0:
		return "0"
	case age == 1:
		return "1"
	case age == 2:
		return "2"
	case age == 3:
		return "3"
	case age == 4:
		return "4"
	case age == 5:
		return "5"
	case age == 6:
		return "6"
	case age == 7:
		return "7"
	case age == 8:
		return "8"
	case age == 9:
		return "9"
	case age >= 10 && age <= 15:
		return "10-15 arası"
	default:
		return "15 üzeri"
	}
}

type horseDetailData struct {
	MaidenSire string
	Owner      string
	Breeder    string
	Trainer    string
	BirthDate  string
	HeightCm   *int
}

func extractHorseDetail(raw json.RawMessage) horseDetailData {
	var out horseDetailData
	if len(raw) == 0 || string(raw) == "null" {
		return out
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return out
	}

	// 1. Check nested profile document from TJK sync
	if pRaw, ok := m["profile"].(map[string]any); ok {
		if v, ok := pRaw["maidenSire"].(string); ok && v != "" {
			out.MaidenSire = strings.TrimSpace(v)
		}
		if v, ok := pRaw["owner"].(string); ok && v != "" {
			out.Owner = strings.TrimSpace(v)
		}
		if v, ok := pRaw["grower"].(string); ok && v != "" {
			out.Breeder = strings.TrimSpace(v)
		}
		if v, ok := pRaw["birthDate"].(string); ok && v != "" {
			out.BirthDate = strings.TrimSpace(v)
		}
	}

	// 2. Direct keys
	if out.MaidenSire == "" {
		for _, k := range []string{"maidenSire", "damsire", "damSire", "anneBabasi"} {
			if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
				out.MaidenSire = strings.TrimSpace(v)
				break
			}
		}
	}
	if out.Breeder == "" {
		for _, k := range []string{"breeder", "yetistirici", "grower"} {
			if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
				out.Breeder = strings.TrimSpace(v)
				break
			}
		}
	}
	if out.Owner == "" {
		for _, k := range []string{"owner", "sahip", "ownersText"} {
			if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
				out.Owner = strings.TrimSpace(v)
				break
			}
		}
	}
	if out.Trainer == "" {
		for _, k := range []string{"trainer", "antrenor"} {
			if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
				out.Trainer = strings.TrimSpace(v)
				break
			}
		}
	}
	if out.BirthDate == "" {
		if v, ok := m["birthDate"].(string); ok && strings.TrimSpace(v) != "" {
			out.BirthDate = strings.TrimSpace(v)
		}
	}
	if v, ok := m["heightCm"].(float64); ok && v > 0 {
		h := int(v)
		out.HeightCm = &h
	}

	return out
}

// EnrichHorseProperties merges normalized TJK horse data into an advert's properties map.
// Existing non-empty values provided in rawProps take precedence.
func EnrichHorseProperties(category domaincatalog.Category, h domainhorse.Horse, rawProps json.RawMessage, now time.Time) (json.RawMessage, error) {
	props := make(map[string]any)
	if len(rawProps) > 0 && string(rawProps) != "null" {
		if err := json.Unmarshal(rawProps, &props); err != nil {
			props = make(map[string]any)
		}
	}

	isStud := IsStudCategory(category.Slug)
	detail := extractHorseDetail(h.Detail)

	setIfEmpty := func(key string, val any) {
		if val == nil {
			return
		}
		if s, ok := val.(string); ok && strings.TrimSpace(s) == "" {
			return
		}
		if current, exists := props[key]; !exists || current == nil || current == "" {
			props[key] = val
		}
	}

	// Normalizations
	var normBreed, normGender, normCoat, normAge string
	if h.Breed != nil && *h.Breed != "" {
		normBreed = NormalizeHorseBreed(*h.Breed, isStud, category.Slug)
	} else if isStud {
		normBreed = NormalizeHorseBreed("", isStud, category.Slug)
	}

	if h.Gender != nil && *h.Gender != "" {
		normGender = NormalizeHorseGender(*h.Gender)
	}

	if h.Coat != nil && *h.Coat != "" {
		normCoat = NormalizeHorseCoat(*h.Coat)
	}

	if h.BirthYear != nil && *h.BirthYear > 0 {
		normAge = NormalizeHorseAge(*h.BirthYear, now, isStud)
	}

	var sire, dam string
	if h.SireName != nil {
		sire = strings.TrimSpace(*h.SireName)
	}
	if h.DamName != nil {
		dam = strings.TrimSpace(*h.DamName)
	}

	if isStud {
		// Aşım Hizmetleri Property Names
		setIfEmpty("STALLION_BREED", normBreed)
		setIfEmpty("STALLION_AGE", normAge)
		setIfEmpty("COAT_COLOR", normCoat)
		setIfEmpty("HORSE_GENDER", "Erkek")
		setIfEmpty("gender", "Erkek")
		setIfEmpty("studHorseName", strings.TrimSpace(h.OriginalName))
		setIfEmpty("studSire", sire)
		setIfEmpty("studDam", dam)
		setIfEmpty("studDamsire", detail.MaidenSire)
		setIfEmpty("TJK_NUMBER", strings.TrimSpace(h.TJKNumber))
		setIfEmpty("BREEDER", detail.Breeder)
		setIfEmpty("TRAINER", detail.Trainer)
	} else {
		// Satılık Atlar Property Names
		setIfEmpty("HORSE_BREED", normBreed)
		setIfEmpty("HORSE_AGE", normAge)
		setIfEmpty("HORSE_GENDER", normGender)
		setIfEmpty("COAT_COLOR", normCoat)
		setIfEmpty("SIRE", sire)
		setIfEmpty("DAM", dam)
		setIfEmpty("DAMSIRE", detail.MaidenSire)
		setIfEmpty("REGISTERED_NAME", strings.TrimSpace(h.OriginalName))
		setIfEmpty("HORSE_NAME", strings.TrimSpace(h.OriginalName))
		setIfEmpty("TJK_NUMBER", strings.TrimSpace(h.TJKNumber))
		setIfEmpty("BREEDER", detail.Breeder)
		setIfEmpty("TRAINER", detail.Trainer)
		setIfEmpty("OWNER", detail.Owner)
		setIfEmpty("BIRTH_DATE", detail.BirthDate)
		if detail.HeightCm != nil {
			setIfEmpty("HEIGHT_CM", *detail.HeightCm)
		}
	}

	encoded, err := json.Marshal(props)
	if err != nil {
		return rawProps, fmt.Errorf("marshal enriched properties: %w", err)
	}
	return encoded, nil
}
