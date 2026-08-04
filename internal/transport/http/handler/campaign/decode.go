package campaign

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	appcampaign "github.com/hkizilbulak/haradan-be/internal/application/campaign"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
)

var updateCampaignAllowed = map[string]struct{}{
	"expectedVersion":   {},
	"name":              {},
	"eventType":         {},
	"sourcePackageCode": {},
	"targetPackageCode": {},
	"title":             {},
	"description":       {},
	"emailSubject":      {},
	"emailHeading":      {},
	"emailBody":         {},
	"ctaLabel":          {},
	"ctaUrl":            {},
	"badgeText":         {},
	"imageAssetId":      {},
	"originalPrice":     {},
	"campaignPrice":     {},
	"currencyCode":      {},
	"startsAt":          {},
	"endsAt":            {},
	"isActive":          {},
}

func decodeUpdateCampaignInput(
	c *gin.Context,
	actorID, campaignID uuid.UUID,
) (appcampaign.UpdateCampaignInput, error) {
	raw, err := bind.PatchObject(c, updateCampaignAllowed)
	if err != nil {
		return appcampaign.UpdateCampaignInput{}, err
	}
	expectedVersion, err := bind.RequireExpectedVersion(raw)
	if err != nil {
		return appcampaign.UpdateCampaignInput{}, err
	}
	in := appcampaign.UpdateCampaignInput{
		ActorUserID:     actorID,
		CampaignID:      campaignID,
		ExpectedVersion: expectedVersion,
	}
	if v, ok := raw["name"]; ok {
		if bind.IsJSONNull(v) {
			return appcampaign.UpdateCampaignInput{}, apperr.Validation("Ad zorunludur.")
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.Name = &s
	}
	if v, ok := raw["eventType"]; ok {
		if bind.IsJSONNull(v) {
			return appcampaign.UpdateCampaignInput{}, apperr.Validation("Olay tipi geçersiz.")
		}
		var et generated.CampaignEventType
		if err := json.Unmarshal(v, &et); err != nil {
			return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		parsed := domaincampaign.CampaignEventType(et)
		in.EventType = &parsed
	}
	if v, ok := raw["sourcePackageCode"]; ok {
		in.SourcePackageCodeSet = true
		if !bind.IsJSONNull(v) {
			var code generated.PackageCode
			if err := json.Unmarshal(v, &code); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			s := string(code)
			in.SourcePackageCode = &s
		}
	}
	if v, ok := raw["targetPackageCode"]; ok {
		in.TargetPackageCodeSet = true
		if !bind.IsJSONNull(v) {
			var code generated.PackageCode
			if err := json.Unmarshal(v, &code); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			s := string(code)
			in.TargetPackageCode = &s
		}
	}
	if v, ok := raw["title"]; ok {
		if bind.IsJSONNull(v) {
			return appcampaign.UpdateCampaignInput{}, apperr.Validation("Başlık zorunludur.")
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.Title = &s
	}
	if v, ok := raw["description"]; ok {
		in.DescriptionSet = true
		if !bind.IsJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.Description = &s
		}
	}
	if v, ok := raw["emailSubject"]; ok {
		in.EmailSubjectSet = true
		if !bind.IsJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.EmailSubject = &s
		}
	}
	if v, ok := raw["emailHeading"]; ok {
		in.EmailHeadingSet = true
		if !bind.IsJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.EmailHeading = &s
		}
	}
	if v, ok := raw["emailBody"]; ok {
		in.EmailBodySet = true
		if !bind.IsJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.EmailBody = &s
		}
	}
	if v, ok := raw["ctaLabel"]; ok {
		in.CTALabelSet = true
		if !bind.IsJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.CTALabel = &s
		}
	}
	if v, ok := raw["ctaUrl"]; ok {
		in.CTAURLSet = true
		if !bind.IsJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.CTAURL = &s
		}
	}
	if v, ok := raw["badgeText"]; ok {
		in.BadgeTextSet = true
		if !bind.IsJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.BadgeText = &s
		}
	}
	if v, ok := raw["imageAssetId"]; ok {
		in.ImageAssetIDSet = true
		if !bind.IsJSONNull(v) {
			var id openapi_types.UUID
			if err := json.Unmarshal(v, &id); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			uid := uuid.UUID(id)
			in.ImageAssetID = &uid
		}
	}
	if v, ok := raw["originalPrice"]; ok {
		if bind.IsJSONNull(v) {
			in.ClearOriginalPrice = true
		} else {
			var m generated.Money
			if err := json.Unmarshal(v, &m); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			amount := int64(m.AmountMinor)
			in.DisplayOriginalPriceAmountMinor = &amount
			if in.CurrencyCode == nil {
				cur := m.Currency
				in.CurrencyCode = &cur
			}
		}
	}
	if v, ok := raw["campaignPrice"]; ok {
		if bind.IsJSONNull(v) {
			in.ClearCampaignPrice = true
		} else {
			var m generated.Money
			if err := json.Unmarshal(v, &m); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			amount := int64(m.AmountMinor)
			in.DisplayCampaignPriceAmountMinor = &amount
			if in.CurrencyCode == nil {
				cur := m.Currency
				in.CurrencyCode = &cur
			}
		}
	}
	if v, ok := raw["currencyCode"]; ok {
		if bind.IsJSONNull(v) {
			return appcampaign.UpdateCampaignInput{}, apperr.Validation("Para birimi geçersiz.")
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.CurrencyCode = &s
	}
	if v, ok := raw["startsAt"]; ok {
		if bind.IsJSONNull(v) {
			return appcampaign.UpdateCampaignInput{}, apperr.Validation("startsAt null olamaz.")
		}
		var t time.Time
		if err := json.Unmarshal(v, &t); err != nil {
			return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.StartsAt = &t
	}
	if v, ok := raw["endsAt"]; ok {
		in.EndsAtSet = true
		if !bind.IsJSONNull(v) {
			var t time.Time
			if err := json.Unmarshal(v, &t); err != nil {
				return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.EndsAt = &t
		}
	}
	if v, ok := raw["isActive"]; ok {
		if bind.IsJSONNull(v) {
			return appcampaign.UpdateCampaignInput{}, apperr.Validation("isActive null olamaz.")
		}
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return appcampaign.UpdateCampaignInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.IsActive = &b
	}
	return in, nil
}
