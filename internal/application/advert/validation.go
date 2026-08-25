package advert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
)

const (
	// maxTitleRunes matches the hrd_adverts.title varchar(200) column.
	maxTitleRunes  = 200
	invalidRequest = "Geçersiz istek."
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// propertyMode selects between partial draft validation and full submit validation.
type propertyMode int

const (
	propertyModeDraft propertyMode = iota + 1
	propertyModeSubmit
)

// validateMoney enforces the price pair rule: null, or both halves valid.
func validateMoney(field string, in *MoneyInput) (*domainadvert.Money, error) {
	if in == nil {
		return nil, nil
	}
	if in.AmountMinor == nil && in.Currency == nil {
		return nil, nil
	}
	if in.AmountMinor == nil || in.Currency == nil {
		return nil, apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   field,
			Message: "Fiyat için tutar ve para birimi birlikte gönderilmelidir.",
		})
	}
	if *in.AmountMinor < 0 {
		return nil, apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   field + ".amountMinor",
			Message: "Tutar negatif olamaz.",
		})
	}
	currency := strings.TrimSpace(*in.Currency)
	if !currencyPattern.MatchString(currency) {
		return nil, apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   field + ".currency",
			Message: "Para birimi üç büyük harf olmalıdır.",
		})
	}
	return &domainadvert.Money{AmountMinor: *in.AmountMinor, Currency: currency}, nil
}

// normalizeTitle trims the title and rejects over-long values. A blank title
// normalizes to null so partial drafts stay storable.
func normalizeTitle(field string, in *string) (*string, error) {
	if in == nil {
		return nil, nil
	}
	v := strings.TrimSpace(*in)
	if v == "" {
		return nil, nil
	}
	if len([]rune(v)) > maxTitleRunes {
		return nil, apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   field,
			Message: fmt.Sprintf("Başlık en fazla %d karakter olabilir.", maxTitleRunes),
		})
	}
	return &v, nil
}

// normalizeDescription trims the description; blank normalizes to null.
func normalizeDescription(in *string) *string {
	if in == nil {
		return nil
	}
	v := strings.TrimSpace(*in)
	if v == "" {
		return nil
	}
	return &v
}

func normalizePropertyKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func resolvePropertyAlias(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "facilitygrasspaddock", "grasspaddock", "grass_paddock":
		return "GRASS_PADDOCK"
	case "facilitysandpaddock", "sandpaddock", "sand_paddock":
		return "SAND_PADDOCK"
	case "facilitystallionpaddock", "stallionpaddock", "stallion_paddock":
		return "STALLION_PADDOCK"
	case "facilitytrainingtrack", "trainingtrack", "training_track":
		return "TRAINING_TRACK"
	case "facilityveterinarian", "vet", "vet_service":
		return "VET"
	case "facilityfarrier", "farrier", "farrier_service":
		return "FARRIER"
	case "facilityfoalingbarn", "foalingbarn", "foaling_barn":
		return "FOALING_BARN"
	case "studbreed", "stallionbreed", "stallion_breed":
		return "STALLION_BREED"
	case "studage", "stallionage", "stallion_age":
		return "STALLION_AGE"
	case "studcoatcolor", "coatcolor", "coat_color":
		return "COAT_COLOR"
	case "studhorsename", "horsename", "horse_name":
		return "HORSE_NAME"
	case "studsire", "sire":
		return "SIRE"
	case "studdam", "dam":
		return "DAM"
	case "studdamsire", "damsire":
		return "DAMSIRE"
	case "companyname", "company_name":
		return "COMPANY_NAME"
	case "websiteurl", "website_url":
		return "WEBSITE_URL"
	default:
		return ""
	}
}

func matchPropertyDef(defs []domaincatalog.Property, key string) (domaincatalog.Property, bool) {
	cleanKey := strings.TrimSpace(key)
	if cleanKey == "" {
		return domaincatalog.Property{}, false
	}
	// 1. Exact match
	for _, d := range defs {
		if d.Code == cleanKey {
			return d, true
		}
	}
	// 2. Case-insensitive match
	for _, d := range defs {
		if strings.EqualFold(d.Code, cleanKey) {
			return d, true
		}
	}
	// 3. Normalized key match (stripping underscores, hyphens, casing)
	normKey := normalizePropertyKey(cleanKey)
	for _, d := range defs {
		if normalizePropertyKey(d.Code) == normKey {
			return d, true
		}
	}
	// 4. Match against Title (both exact, case-insensitive, and normalized)
	for _, d := range defs {
		if strings.EqualFold(d.Title, cleanKey) || normalizePropertyKey(d.Title) == normKey {
			return d, true
		}
	}
	// 5. Common aliases mapping
	aliasTarget := resolvePropertyAlias(cleanKey)
	if aliasTarget != "" {
		for _, d := range defs {
			if strings.EqualFold(d.Code, aliasTarget) || normalizePropertyKey(d.Code) == normalizePropertyKey(aliasTarget) {
				return d, true
			}
		}
	}
	return domaincatalog.Property{}, false
}

func isPhoneProperty(code string) bool {
	norm := strings.ToLower(strings.TrimSpace(code))
	return norm == "sellerphone" || norm == "phone" || norm == "seller_phone" || norm == "telefon"
}

// validateDynamicProperties checks a property map against the category form
// definition. Only active, form-visible property codes are accepted; unknown
// keys are rejected. Draft mode skips is_required, submit mode enforces it.
func validateDynamicProperties(
	defs []domaincatalog.Property,
	raw json.RawMessage,
	mode propertyMode,
) (json.RawMessage, error) {
	values, err := decodePropertyObject(raw)
	if err != nil {
		return nil, err
	}

	var fieldErrors []apperr.FieldError
	normalized := make(map[string]json.RawMessage, len(values))
	for code, value := range values {
		if isPhoneProperty(code) {
			if isJSONNull(value) {
				continue
			}
			var str string
			if err := json.Unmarshal(value, &str); err == nil && strings.TrimSpace(str) != "" {
				trimmed, _ := json.Marshal(strings.TrimSpace(str))
				normalized["sellerPhone"] = trimmed
			}
			continue
		}
		def, ok := matchPropertyDef(defs, code)
		if !ok {
			fieldErrors = append(fieldErrors, apperr.FieldError{
				Field:   "properties." + code,
				Message: "Bu kategori için tanımlı olmayan özellik.",
			})
			continue
		}
		if isJSONNull(value) {
			continue
		}
		clean, verr := normalizePropertyValue(def, value)
		if verr != nil {
			fieldErrors = append(fieldErrors, *verr)
			continue
		}
		// A blank value normalizes to "unset" and must not satisfy is_required.
		if clean == nil {
			continue
		}
		normalized[def.Code] = clean
	}

	if mode == propertyModeSubmit {
		for _, def := range defs {
			if !def.IsRequired {
				continue
			}
			if _, ok := normalized[def.Code]; !ok {
				fieldErrors = append(fieldErrors, apperr.FieldError{
					Field:   "properties." + def.Code,
					Message: "Bu özellik zorunludur.",
				})
			}
		}
	}

	if len(fieldErrors) > 0 {
		return nil, apperr.Validation(invalidRequest, fieldErrors...)
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("encode properties: %w", err))
	}
	return encoded, nil
}

func decodePropertyObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return map[string]json.RawMessage{}, nil
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &out); err != nil || out == nil {
		return nil, apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "properties",
			Message: "Özellikler bir nesne olmalıdır.",
		})
	}
	return out, nil
}

func normalizePropertyValue(def domaincatalog.Property, value json.RawMessage) (json.RawMessage, *apperr.FieldError) {
	field := "properties." + def.Code
	switch def.DataType {
	case "STRING", "TEXT":
		s, ok := decodeString(value)
		if !ok {
			return nil, &apperr.FieldError{Field: field, Message: "Metin değeri bekleniyor."}
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		encoded, err := json.Marshal(s)
		if err != nil {
			return nil, &apperr.FieldError{Field: field, Message: "Metin değeri bekleniyor."}
		}
		return encoded, nil

	case "INTEGER", "YEAR":
		num, ok := decodeNumber(value)
		if !ok {
			return nil, &apperr.FieldError{Field: field, Message: "Tam sayı değeri bekleniyor."}
		}
		if _, err := strconv.ParseInt(num.String(), 10, 64); err != nil {
			return nil, &apperr.FieldError{Field: field, Message: "Tam sayı değeri bekleniyor."}
		}
		return json.RawMessage(num.String()), nil

	case "DECIMAL":
		num, ok := decodeNumber(value)
		if !ok {
			return nil, &apperr.FieldError{Field: field, Message: "Sayısal değer bekleniyor."}
		}
		if _, err := strconv.ParseFloat(num.String(), 64); err != nil {
			return nil, &apperr.FieldError{Field: field, Message: "Sayısal değer bekleniyor."}
		}
		return json.RawMessage(num.String()), nil

	case "BOOLEAN":
		var b bool
		if err := json.Unmarshal(value, &b); err != nil {
			var s string
			if sErr := json.Unmarshal(value, &s); sErr == nil {
				s = strings.ToLower(strings.TrimSpace(s))
				if s == "true" || s == "1" {
					b = true
				} else if s == "false" || s == "0" {
					b = false
				} else {
					return nil, &apperr.FieldError{Field: field, Message: "Doğru/yanlış değeri bekleniyor."}
				}
			} else {
				return nil, &apperr.FieldError{Field: field, Message: "Doğru/yanlış değeri bekleniyor."}
			}
		}
		return json.RawMessage(strconv.FormatBool(b)), nil

	case "SINGLE_SELECT":
		s, ok := decodeString(value)
		if !ok {
			return nil, &apperr.FieldError{Field: field, Message: "Seçenek değeri bekleniyor."}
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		canonical, ok := matchOption(def.Options, s)
		if !ok {
			return nil, &apperr.FieldError{Field: field, Message: "Geçersiz seçenek."}
		}
		encoded, err := json.Marshal(canonical)
		if err != nil {
			return nil, &apperr.FieldError{Field: field, Message: "Seçenek değeri bekleniyor."}
		}
		return encoded, nil
	}

	return nil, &apperr.FieldError{Field: field, Message: "Desteklenmeyen özellik tipi."}
}

// matchOption matches a SINGLE_SELECT value against the option list and returns
// the canonical option value.
func matchOption(options json.RawMessage, value string) (string, bool) {
	if len(bytes.TrimSpace(options)) == 0 {
		return value, true
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(options, &entries); err != nil {
		return value, true
	}
	if len(entries) == 0 {
		return value, true
	}
	valNorm := strings.TrimSpace(value)
	for _, entry := range entries {
		if s, ok := decodeString(entry); ok {
			if strings.EqualFold(strings.TrimSpace(s), valNorm) {
				return s, true
			}
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(entry, &obj); err != nil {
			continue
		}
		canonicalVal := ""
		if rawVal, ok := obj["value"]; ok {
			if s, ok := decodeString(rawVal); ok {
				canonicalVal = s
			}
		}
		if canonicalVal == "" {
			if rawCode, ok := obj["code"]; ok {
				if s, ok := decodeString(rawCode); ok {
					canonicalVal = s
				}
			}
		}
		for _, key := range []string{"value", "code", "label", "title", "name"} {
			if raw, ok := obj[key]; ok {
				if s, ok := decodeString(raw); ok && strings.EqualFold(strings.TrimSpace(s), valNorm) {
					if canonicalVal != "" {
						return canonicalVal, true
					}
					return s, true
				}
			}
		}
	}
	return "", false
}

func decodeString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

func decodeNumber(raw json.RawMessage) (json.Number, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", false
	}
	if num, ok := v.(json.Number); ok {
		return num, true
	}
	if s, ok := v.(string); ok {
		s = strings.TrimSpace(s)
		if s == "" {
			return "", false
		}
		return json.Number(s), true
	}
	return "", false
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
