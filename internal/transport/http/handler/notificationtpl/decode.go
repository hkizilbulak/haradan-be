package notificationtpl

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
)

var updateTemplateAllowed = map[string]struct{}{
	"expectedVersion":      {},
	"name":                 {},
	"inAppTitleTemplate":   {},
	"inAppBodyTemplate":    {},
	"resendTemplateId":     {},
	"emailSubjectFallback": {},
	"isActive":             {},
}

func decodeUpdateTemplateInput(
	c *gin.Context,
	actorID uuid.UUID,
	eventType domainnotification.TemplateEventType,
) (appnotification.UpdateTemplateInput, error) {
	raw, err := bind.PatchObject(c, updateTemplateAllowed)
	if err != nil {
		return appnotification.UpdateTemplateInput{}, err
	}
	expectedVersion, err := bind.RequireExpectedVersion(raw)
	if err != nil {
		return appnotification.UpdateTemplateInput{}, err
	}
	in := appnotification.UpdateTemplateInput{
		ActorUserID:     actorID,
		EventType:       eventType,
		ExpectedVersion: expectedVersion,
	}
	if v, ok := raw["name"]; ok {
		if bind.IsJSONNull(v) {
			return appnotification.UpdateTemplateInput{}, apperr.Validation("Ad zorunludur.")
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return appnotification.UpdateTemplateInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.Name = &s
	}
	if v, ok := raw["inAppTitleTemplate"]; ok {
		if bind.IsJSONNull(v) {
			return appnotification.UpdateTemplateInput{}, apperr.Validation("Başlık şablonu zorunludur.")
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return appnotification.UpdateTemplateInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.InAppTitleTemplate = &s
	}
	if v, ok := raw["inAppBodyTemplate"]; ok {
		if bind.IsJSONNull(v) {
			return appnotification.UpdateTemplateInput{}, apperr.Validation("Gövde şablonu zorunludur.")
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return appnotification.UpdateTemplateInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.InAppBodyTemplate = &s
	}
	if v, ok := raw["resendTemplateId"]; ok {
		in.ResendTemplateIDSet = true
		if !bind.IsJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appnotification.UpdateTemplateInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.ResendTemplateID = &s
		}
	}
	if v, ok := raw["emailSubjectFallback"]; ok {
		in.EmailSubjectFallbackSet = true
		if !bind.IsJSONNull(v) {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appnotification.UpdateTemplateInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
			}
			in.EmailSubjectFallback = &s
		}
	}
	if v, ok := raw["isActive"]; ok {
		if bind.IsJSONNull(v) {
			return appnotification.UpdateTemplateInput{}, apperr.Validation("isActive null olamaz.")
		}
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return appnotification.UpdateTemplateInput{}, apperr.BadRequest(apperr.CodeValidation, bind.MalformedBodyMessage)
		}
		in.IsActive = &b
	}
	return in, nil
}
