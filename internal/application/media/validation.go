package media

import (
	"strings"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

const (
	invalidRequest = "Geçersiz istek."

	assetNotFoundMessage       = "Görsel bulunamadı."
	advertNotFoundMessage      = "İlan bulunamadı."
	deletedAdvertMessage       = "Silinmiş ilan güncellenemez."
	advertNotEditableMessage   = "İlan bu durumda medya düzenlemeye kapalı."
	staleMediaVersionMessage   = "İlan görselleri başka bir yerden güncellendi; sayfayı yenileyin."
	assetNotAttachableMessage  = "Bu görsel ilana eklenemez."
	assetAlreadyAttached       = "Bu görsel ilana zaten ekli."
	assetNotAttachedMessage    = "Bu görsel ilana ekli değil."
	displayOrderTakenMessage   = "Bu sıra numarası zaten kullanılıyor."
	uploadNotCompletedMessage  = "Yükleme tamamlanmadı; dosyayı tekrar yükleyin."
	assetNotConfirmableMessage = "Bu görsel için yükleme tamamlanamaz."

	// mediaNotConfiguredMessage is returned while the MIME allowlist or the byte
	// ceiling is unset: accepting an upload under invented limits would be worse
	// than reporting the dependency as unavailable.
	mediaNotConfiguredMessage = "Görsel yükleme şu anda yapılandırılmadı."
)

// normalizeContentTypes lowercases and trims a configured MIME allowlist,
// dropping blanks and duplicates.
func normalizeContentTypes(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// validateDeclaredContentType checks the client hint against the allowlist. The
// declared type is not canonical; the worker re-derives the real one from bytes.
func validateDeclaredContentType(allowed []string, declared *string) (string, error) {
	if declared == nil {
		return "", nil
	}
	value := strings.ToLower(strings.TrimSpace(*declared))
	if value == "" {
		return "", nil
	}
	for _, candidate := range allowed {
		if candidate == value {
			return value, nil
		}
	}
	return "", apperr.Validation(invalidRequest, apperr.FieldError{
		Field:   "declaredContentType",
		Message: "Bu dosya türü desteklenmiyor.",
	})
}

// validateDeclaredByteSize checks the client size hint against the ceiling.
func validateDeclaredByteSize(maxByteSize int64, declared *int64) error {
	ceiling := effectiveMaxByteSize(maxByteSize)
	if declared == nil {
		return nil
	}
	if *declared <= 0 {
		return apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "declaredByteSize",
			Message: "Dosya boyutu sıfırdan büyük olmalıdır.",
		})
	}
	if *declared > ceiling {
		return apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "declaredByteSize",
			Message: "Dosya izin verilen boyuttan büyük.",
		})
	}
	return nil
}

// effectiveMaxByteSize applies the 64 MiB security ceiling. A non-positive
// configured limit falls back to MaxUploadBytes.
func effectiveMaxByteSize(configured int64) int64 {
	if configured <= 0 || configured > domainmedia.MaxUploadBytes {
		return domainmedia.MaxUploadBytes
	}
	return configured
}

// validateStoredObject enforces provider-reported object metadata against the
// configured allowlist and byte ceiling. Client-declared hints are never used
// here; an empty ContentType is deferred to the worker, which re-derives type
// from bytes. Checksum comparison is not available until a real storage
// adapter exposes it.
func validateStoredObject(allowed []string, maxByteSize int64, info ObjectInfo) error {
	ceiling := effectiveMaxByteSize(maxByteSize)
	if !info.Exists {
		return apperr.InvalidState(uploadNotCompletedMessage)
	}
	if info.ByteSize <= 0 {
		return apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "byteSize",
			Message: "Yüklenen dosya boş.",
		})
	}
	if info.ByteSize > ceiling {
		return apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "byteSize",
			Message: "Dosya izin verilen boyuttan büyük.",
		})
	}
	contentType := strings.ToLower(strings.TrimSpace(info.ContentType))
	if contentType == "" {
		return nil
	}
	for _, candidate := range allowed {
		if candidate == contentType {
			return nil
		}
	}
	return apperr.Validation(invalidRequest, apperr.FieldError{
		Field:   "contentType",
		Message: "Bu dosya türü desteklenmiyor.",
	})
}

func requireExpectedMediaVersion(v int) error {
	if v < 1 {
		return apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "expectedMediaVersion",
			Message: "Sürüm numarası 1 veya daha büyük olmalıdır.",
		})
	}
	return nil
}

func requireAssetID(id uuid.UUID) error {
	if id == uuid.Nil {
		return apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "assetId",
			Message: "Görsel zorunludur.",
		})
	}
	return nil
}

func validateDisplayOrder(displayOrder *int) error {
	if displayOrder == nil {
		return nil
	}
	if *displayOrder < 0 {
		return apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "displayOrder",
			Message: "Sıra numarası negatif olamaz.",
		})
	}
	return nil
}

// validateOrderedAssetIDs rejects an empty or duplicated ordering before any row
// is touched.
func validateOrderedAssetIDs(ids []uuid.UUID) error {
	if len(ids) == 0 {
		return apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "orderedAssetIds",
			Message: "Sıralama listesi boş olamaz.",
		})
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return apperr.Validation(invalidRequest, apperr.FieldError{
				Field:   "orderedAssetIds",
				Message: "Geçersiz görsel kimliği.",
			})
		}
		if _, dup := seen[id]; dup {
			return apperr.Validation(invalidRequest, apperr.FieldError{
				Field:   "orderedAssetIds",
				Message: "Aynı görsel listede birden fazla kez yer alamaz.",
			})
		}
		seen[id] = struct{}{}
	}
	return nil
}

// requireExactRelationSet enforces that a reorder covers exactly the attached
// assets: no additions, no omissions.
func requireExactRelationSet(rows []RelationRow, ids []uuid.UUID) error {
	if len(rows) != len(ids) {
		return apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "orderedAssetIds",
			Message: "Sıralama listesi ilanın bütün görsellerini içermelidir.",
		})
	}
	attached := make(map[uuid.UUID]struct{}, len(rows))
	for _, row := range rows {
		attached[row.Relation.AssetID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := attached[id]; !ok {
			return apperr.Validation(invalidRequest, apperr.FieldError{
				Field:   "orderedAssetIds",
				Message: "Sıralama listesi ilana ekli olmayan görsel içeriyor.",
			})
		}
	}
	return nil
}
