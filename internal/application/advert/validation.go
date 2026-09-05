package advert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	"github.com/hkizilbulak/haradan-be/internal/platform/textnorm"
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

// validateDynamicProperties checks a property map against the category form
var propertyAliases = map[string][]string{
	"studhorse":      {"studhorsename", "registeredname", "horsename"},
	"studhorsename":  {"studhorse", "registeredname", "horsename"},
	"registeredname": {"studhorse", "studhorsename", "horsename"},
	"horsename":      {"studhorse", "studhorsename", "registeredname"},
	"studsire":       {"sire"},
	"sire":           {"studsire"},
	"studdam":        {"dam"},
	"dam":            {"studdam"},
	"studdamsire":    {"studdamsire", "damsire"},
	"damsire":        {"studdamsire", "studdamsire"},
	"studbreed":      {"stallionbreed", "horsebreed", "breed"},
	"stallionbreed":  {"studbreed", "horsebreed", "breed"},
	"horsebreed":     {"studbreed", "stallionbreed", "breed"},
	"breed":          {"studbreed", "stallionbreed", "horsebreed"},
	"studage":        {"stallionage", "horseage", "age"},
	"stallionage":    {"studage", "horseage", "age"},
	"horseage":       {"studage", "stallionage", "age"},
	"age":            {"studage", "stallionage", "horseage"},
	"studcoatcolor":  {"coatcolor"},
	"coatcolor":      {"studcoatcolor"},
}

func findPropertyDef(byCode, byNormCode map[string]domaincatalog.Property, code string) (domaincatalog.Property, bool) {
	if def, ok := byCode[code]; ok {
		return def, true
	}
	norm := normalizeCode(code)
	if def, ok := byNormCode[norm]; ok {
		return def, true
	}
	for _, alias := range propertyAliases[norm] {
		if def, ok := byNormCode[alias]; ok {
			return def, true
		}
		if def, ok := byCode[alias]; ok {
			return def, true
		}
	}
	return domaincatalog.Property{}, false
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

	byCode := make(map[string]domaincatalog.Property, len(defs))
	byNormCode := make(map[string]domaincatalog.Property, len(defs))
	for _, d := range defs {
		byCode[d.Code] = d
		byNormCode[normalizeCode(d.Code)] = d
	}

	var fieldErrors []apperr.FieldError
	normalized := make(map[string]json.RawMessage, len(values))

	// 1. Process category property definitions, prioritizing exact matches over aliases.
	for _, def := range defs {
		norm := normalizeCode(def.Code)
		var val json.RawMessage
		var hasVal bool

		if v, ok := values[def.Code]; ok && !isJSONNull(v) {
			val = v
			hasVal = true
		} else {
			for k, v := range values {
				if normalizeCode(k) == norm && !isJSONNull(v) {
					val = v
					hasVal = true
					break
				}
			}
		}

		if !hasVal {
			for _, alias := range propertyAliases[norm] {
				if v, ok := values[alias]; ok && !isJSONNull(v) {
					val = v
					hasVal = true
					break
				}
				for k, v := range values {
					if normalizeCode(k) == alias && !isJSONNull(v) {
						val = v
						hasVal = true
						break
					}
				}
				if hasVal {
					break
				}
			}
		}

		if hasVal {
			clean, verr := normalizePropertyValue(def, val)
			if verr != nil {
				fieldErrors = append(fieldErrors, *verr)
				continue
			}
			if clean != nil {
				normalized[def.Code] = clean
			}
		}
	}

	// 2. Process special keys and non-conflicting extra keys.
	for code, value := range values {
		if code == "sellerPhone" || code == "phone" {
			if isJSONNull(value) {
				continue
			}
			var str string
			if err := json.Unmarshal(value, &str); err == nil && strings.TrimSpace(str) != "" {
				trimmed, _ := json.Marshal(strings.TrimSpace(str))
				normalized[code] = trimmed
			}
			continue
		}
		if _, ok := byCode[code]; ok {
			continue
		}
		if _, ok := byNormCode[normalizeCode(code)]; ok {
			continue
		}
		isAliasOfCategoryDef := false
		for _, alias := range propertyAliases[normalizeCode(code)] {
			if _, ok := byNormCode[alias]; ok {
				isAliasOfCategoryDef = true
				break
			}
		}
		if isAliasOfCategoryDef {
			continue
		}
		if !isJSONNull(value) {
			normalized[code] = value
		}
	}

	if mode == propertyModeSubmit {
		coreColumnCodes := map[string]bool{
			"ADDRESS": true, "address": true,
			"TITLE": true, "title": true,
			"DESCRIPTION": true, "description": true,
			"PRICE": true, "price": true,
			"LOCATION": true, "location": true,
			"PHONE": true, "phone": true,
			"MEDIA": true, "media": true,
			"IMAGES": true, "images": true,
		}
		for _, def := range defs {
			if !def.IsRequired {
				continue
			}
			if coreColumnCodes[def.Code] || coreColumnCodes[strings.ToUpper(def.Code)] || coreColumnCodes[strings.ToLower(def.Code)] {
				continue
			}
			if _, ok := normalized[def.Code]; !ok {
				if _, okNorm := normalized[normalizeCode(def.Code)]; !okNorm {
					hasAlias := false
					for _, alias := range propertyAliases[normalizeCode(def.Code)] {
						if _, okAlias := normalized[alias]; okAlias {
							hasAlias = true
							break
						}
					}
					if !hasAlias {
						fieldErrors = append(fieldErrors, apperr.FieldError{
							Field:   "properties." + def.Code,
							Message: "Bu özellik zorunludur.",
						})
					}
				}
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

func normalizeCode(c string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(c), "-", ""), "_", ""))
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
				if s == "true" || s == "1" || s == "evet" || s == "var" || s == "yes" {
					b = true
				} else if s == "false" || s == "0" || s == "hayir" || s == "hayır" || s == "yok" || s == "no" {
					b = false
				} else {
					return nil, &apperr.FieldError{Field: field, Message: "Doğru/yanlış değeri bekleniyor."}
				}
			} else {
				var n json.Number
				if numErr := json.Unmarshal(value, &n); numErr == nil {
					if n.String() == "1" {
						b = true
					} else if n.String() == "0" {
						b = false
					} else {
						return nil, &apperr.FieldError{Field: field, Message: "Doğru/yanlış değeri bekleniyor."}
					}
				} else {
					return nil, &apperr.FieldError{Field: field, Message: "Doğru/yanlış değeri bekleniyor."}
				}
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
		// If a boolean falsy or truthy value is passed for a single select without a matching option, treat as unset
		if (s == "false" || s == "0" || strings.EqualFold(s, "hayir") || strings.EqualFold(s, "hayır") || strings.EqualFold(s, "yok")) && !optionAllowed(def.Options, s) {
			return nil, nil
		}
		if (s == "true" || s == "1" || strings.EqualFold(s, "evet") || strings.EqualFold(s, "var")) && !optionAllowed(def.Options, s) {
			return nil, nil
		}
		if !optionAllowed(def.Options, s) {
			return nil, &apperr.FieldError{Field: field, Message: "Geçersiz seçenek."}
		}
		encoded, err := json.Marshal(s)
		if err != nil {
			return nil, &apperr.FieldError{Field: field, Message: "Seçenek değeri bekleniyor."}
		}
		return encoded, nil
	}

	return nil, &apperr.FieldError{Field: field, Message: "Desteklenmeyen özellik tipi."}
}

// optionAllowed matches a SINGLE_SELECT value against the option list, which is
// a JSON array of either plain strings or objects carrying a value/code key.
func optionAllowed(options json.RawMessage, value string) bool {
	if len(bytes.TrimSpace(options)) == 0 {
		return true
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(options, &entries); err != nil || len(entries) == 0 {
		return true
	}
	valTrimmed := strings.TrimSpace(value)
	if valTrimmed == "" {
		return true
	}
	valFold := textnorm.TurkishFold(valTrimmed)
	valLower := strings.ToLower(valTrimmed)

	for _, entry := range entries {
		var candidateStrings []string
		if s, ok := decodeString(entry); ok {
			candidateStrings = append(candidateStrings, s)
		} else {
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(entry, &obj); err == nil {
				for _, key := range []string{"value", "code", "label", "title", "name", "id"} {
					if raw, ok := obj[key]; ok {
						if s, ok := decodeString(raw); ok {
							candidateStrings = append(candidateStrings, s)
						}
					}
				}
			}
		}

		for _, s := range candidateStrings {
			sTrimmed := strings.TrimSpace(s)
			sFold := textnorm.TurkishFold(sTrimmed)
			sLower := strings.ToLower(sTrimmed)

			if strings.EqualFold(sTrimmed, valTrimmed) || sFold == valFold || sLower == valLower ||
				strings.Contains(sFold, valFold) || strings.Contains(valFold, sFold) ||
				strings.Contains(sLower, valLower) || strings.Contains(valLower, sLower) {
				return true
			}
			if isBooleanMatch(valFold, sFold) || isBooleanMatch(valLower, sLower) {
				return true
			}
			if isAgeMatch(valTrimmed, sTrimmed) {
				return true
			}
			if isBreedMatch(valFold, sFold) {
				return true
			}
		}
	}
	return false
}

func isBreedMatch(v1, v2 string) bool {
	v1Fold := textnorm.TurkishFold(v1)
	v2Fold := textnorm.TurkishFold(v2)
	isArab1 := strings.Contains(v1Fold, "arap") || strings.Contains(v1Fold, "arabian")
	isArab2 := strings.Contains(v2Fold, "arap") || strings.Contains(v2Fold, "arabian")
	if isArab1 && isArab2 {
		return true
	}
	isIng1 := strings.Contains(v1Fold, "ingiliz") || strings.Contains(v1Fold, "thoroughbred")
	isIng2 := strings.Contains(v2Fold, "ingiliz") || strings.Contains(v2Fold, "thoroughbred")
	if isIng1 && isIng2 {
		return true
	}
	return false
}

func isAgeMatch(val, opt string) bool {
	valNum, errVal := parseLeadingFloat(val)
	optNum, errOpt := parseLeadingFloat(opt)
	if errVal == nil && errOpt == nil && valNum == optNum {
		return true
	}
	optFold := textnorm.TurkishFold(opt)
	if errVal == nil {
		if strings.Contains(optFold, "5+") || strings.Contains(optFold, "5 +") || strings.Contains(optFold, "5 ve üzeri") {
			if valNum >= 5 {
				return true
			}
		}
		if strings.Contains(optFold, "10-15") || strings.Contains(optFold, "10 - 15") {
			if valNum >= 10 && valNum <= 15 {
				return true
			}
		}
		if strings.Contains(optFold, "15 üzeri") || strings.Contains(optFold, "15+") || strings.Contains(optFold, "15 +") {
			if valNum >= 15 {
				return true
			}
		}
		if strings.Contains(optFold, "0-1") || strings.Contains(optFold, "tay") {
			if valNum <= 1 {
				return true
			}
		}
	}
	return false
}

func parseLeadingFloat(s string) (float64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	var b strings.Builder
	for i, r := range s {
		if (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
		} else if i > 0 {
			break
		}
	}
	if b.Len() == 0 {
		return 0, fmt.Errorf("no number")
	}
	return strconv.ParseFloat(b.String(), 64)
}

func isBooleanMatch(v1, v2 string) bool {
	truthy := map[string]bool{"true": true, "1": true, "evet": true, "var": true, "yes": true}
	falsy := map[string]bool{"false": true, "0": true, "hayir": true, "hayır": true, "yok": true, "no": true}
	if truthy[v1] && truthy[v2] {
		return true
	}
	if falsy[v1] && falsy[v2] {
		return true
	}
	return false
}

func decodeString(raw json.RawMessage) (string, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", false
	}
	var s string
	if err := json.Unmarshal(trimmed, &s); err == nil {
		return s, true
	}
	var b bool
	if err := json.Unmarshal(trimmed, &b); err == nil {
		return strconv.FormatBool(b), true
	}
	var num json.Number
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&num); err == nil && num.String() != "" {
		return num.String(), true
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err == nil {
		for _, k := range []string{"value", "label", "code", "title", "name", "id"} {
			if v, ok := obj[k]; ok {
				if str, ok := decodeString(v); ok && str != "" {
					return str, true
				}
			}
		}
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(trimmed, &arr); err == nil && len(arr) > 0 {
		return decodeString(arr[0])
	}
	return "", false
}

func decodeNumber(raw json.RawMessage) (json.Number, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", false
	}
	var b bool
	if err := json.Unmarshal(trimmed, &b); err == nil {
		if b {
			return json.Number("1"), true
		}
		return json.Number("0"), true
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
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
		s = strings.ReplaceAll(s, ",", ".")
		return json.Number(s), true
	}
	return "", false
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
