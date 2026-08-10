package packaging

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/google/uuid"
	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
)

var updatePackageAllowed = map[string]struct{}{
	"expectedVersion":     {},
	"displayName":         {},
	"description":         {},
	"badgeText":           {},
	"benefits":            {},
	"displayPrice":        {},
	"currencyCode":        {},
	"defaultDurationDays": {},
	"allowsUrgent":        {},
	"showcaseEligible":    {},
	"searchPriority":      {},
	"broadcastOnPublish":  {},
	"isActive":            {},
	"sortOrder":           {},
}

func decodeUpdatePackageInput(
	c *gin.Context,
	actorID uuid.UUID,
	packageCode domainpackaging.PackageCode,
) (apppackaging.UpdatePackageInput, error) {
	raw, err := bind.PatchObject(c, updatePackageAllowed)
	if err != nil {
		return apppackaging.UpdatePackageInput{}, err
	}
	expectedVersion, err := bind.RequireExpectedVersion(raw)
	if err != nil {
		return apppackaging.UpdatePackageInput{}, err
	}
	in := apppackaging.UpdatePackageInput{
		ActorUserID:     actorID,
		Code:            packageCode,
		ExpectedVersion: expectedVersion,
	}
	if v, ok := raw["displayName"]; ok {
		if bind.IsJSONNull(v) {
			return apppackaging.UpdatePackageInput{}, apperr.Validation("Görünen ad boş olamaz.")
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.DisplayName = &s
	}
	if v, ok := raw["description"]; ok {
		in.DescriptionSet = true
		if !bind.IsJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.Description = &s
		}
	}
	if v, ok := raw["badgeText"]; ok {
		in.BadgeTextSet = true
		if !bind.IsJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.BadgeText = &s
		}
	}
	if v, ok := raw["benefits"]; ok {
		if bind.IsJSONNull(v) {
			return apppackaging.UpdatePackageInput{}, apperr.Validation("Benefits null olamaz.")
		}
		var benefits []string
		if err := json.Unmarshal(v, &benefits); err != nil {
			return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.Benefits = &benefits
	}
	if v, ok := raw["displayPrice"]; ok {
		in.DisplayPriceSet = true
		if !bind.IsJSONNull(v) {
			var m generated.Money
			if err := json.Unmarshal(v, &m); err != nil {
				return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			amount := int64(m.AmountMinor)
			in.DisplayPriceAmountMinor = &amount
			if in.CurrencyCode == nil {
				cur := m.Currency
				in.CurrencyCode = &cur
			}
		}
	}
	if v, ok := raw["currencyCode"]; ok {
		if bind.IsJSONNull(v) {
			return apppackaging.UpdatePackageInput{}, apperr.Validation("Geçersiz para birimi.")
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.CurrencyCode = &s
	}
	if v, ok := raw["defaultDurationDays"]; ok {
		in.DefaultDurationSet = true
		if !bind.IsJSONNull(v) {
			var d int
			if err := json.Unmarshal(v, &d); err != nil {
				return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.DefaultDurationDays = &d
		}
	}
	if v, ok := raw["allowsUrgent"]; ok {
		if bind.IsJSONNull(v) {
			return apppackaging.UpdatePackageInput{}, apperr.Validation("allowsUrgent null olamaz.")
		}
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.AllowsUrgent = &b
	}
	if v, ok := raw["showcaseEligible"]; ok {
		if bind.IsJSONNull(v) {
			return apppackaging.UpdatePackageInput{}, apperr.Validation("showcaseEligible null olamaz.")
		}
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.ShowcaseEligible = &b
	}
	if v, ok := raw["searchPriority"]; ok {
		if bind.IsJSONNull(v) {
			return apppackaging.UpdatePackageInput{}, apperr.Validation("searchPriority null olamaz.")
		}
		var n int
		if err := json.Unmarshal(v, &n); err != nil {
			return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.SearchPriority = &n
	}
	if v, ok := raw["broadcastOnPublish"]; ok {
		if bind.IsJSONNull(v) {
			return apppackaging.UpdatePackageInput{}, apperr.Validation("broadcastOnPublish null olamaz.")
		}
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.BroadcastOnPublish = &b
	}
	if v, ok := raw["isActive"]; ok {
		if bind.IsJSONNull(v) {
			return apppackaging.UpdatePackageInput{}, apperr.Validation("isActive null olamaz.")
		}
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.IsActive = &b
	}
	if v, ok := raw["sortOrder"]; ok {
		if bind.IsJSONNull(v) {
			return apppackaging.UpdatePackageInput{}, apperr.Validation("sortOrder null olamaz.")
		}
		var n int
		if err := json.Unmarshal(v, &n); err != nil {
			return apppackaging.UpdatePackageInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.SortOrder = &n
	}
	return in, nil
}
