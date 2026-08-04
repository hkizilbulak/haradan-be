// Package notification holds notification template aggregates aligned with migration 00009.
package notification

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// TemplateEventType is the notification template event_type CHECK set.
type TemplateEventType string

const (
	TemplateEventTypeAdvancedAdvertPublished TemplateEventType = "ADVANCED_ADVERT_PUBLISHED"
	TemplateEventTypeUrgentAdvertActivated   TemplateEventType = "URGENT_ADVERT_ACTIVATED"
	TemplateEventTypePackageExpiry10Days     TemplateEventType = "PACKAGE_EXPIRY_10_DAYS"
	TemplateEventTypePackageExpiry3Days      TemplateEventType = "PACKAGE_EXPIRY_3_DAYS"
)

// Valid reports whether t is a known template event type.
func (t TemplateEventType) Valid() bool {
	switch t {
	case TemplateEventTypeAdvancedAdvertPublished,
		TemplateEventTypeUrgentAdvertActivated,
		TemplateEventTypePackageExpiry10Days,
		TemplateEventTypePackageExpiry3Days:
		return true
	}
	return false
}

// ParseTemplateEventType converts an external value into a TemplateEventType.
func ParseTemplateEventType(v string) (TemplateEventType, bool) {
	t := TemplateEventType(strings.TrimSpace(v))
	return t, t.Valid()
}

// NotificationTemplate is the row from hrd_notification_templates.
type NotificationTemplate struct {
	ID                   uuid.UUID
	EventType            TemplateEventType
	Name                 string
	InAppTitleTemplate   string
	InAppBodyTemplate    string
	ResendTemplateID     *string
	EmailSubjectFallback *string
	IsActive             bool
	Version              int
	UpdatedByUserID      *uuid.UUID
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// NonBlankName reports whether name is non-blank after trim.
func NonBlankName(name string) bool {
	return strings.TrimSpace(name) != ""
}

// NonBlankTitleTemplate reports whether in_app_title_template is non-blank after trim.
func NonBlankTitleTemplate(title string) bool {
	return strings.TrimSpace(title) != ""
}

// NonBlankBodyTemplate reports whether in_app_body_template is non-blank after trim.
func NonBlankBodyTemplate(body string) bool {
	return strings.TrimSpace(body) != ""
}
