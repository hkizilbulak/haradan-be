// Package notification holds notification template aggregates aligned with migration 00009.
package notification

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

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

// EventType is the notification event_type CHECK set (same values as TemplateEventType).
type EventType = TemplateEventType

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

// PackageExpiryDayOffset labels a package expiry reminder horizon.
type PackageExpiryDayOffset string

const (
	PackageExpiryDayOffset10D PackageExpiryDayOffset = "10D"
	PackageExpiryDayOffset3D  PackageExpiryDayOffset = "3D"
)

// Valid reports whether o is a known expiry offset label.
func (o PackageExpiryDayOffset) Valid() bool {
	switch o {
	case PackageExpiryDayOffset10D, PackageExpiryDayOffset3D:
		return true
	}
	return false
}

// EventTypeForExpiryOffset maps a day offset to the notification event type.
func EventTypeForExpiryOffset(offset PackageExpiryDayOffset) (EventType, bool) {
	switch offset {
	case PackageExpiryDayOffset10D:
		return TemplateEventTypePackageExpiry10Days, true
	case PackageExpiryDayOffset3D:
		return TemplateEventTypePackageExpiry3Days, true
	default:
		return "", false
	}
}

// EmailStatus is the recipient email delivery state CHECK set (migration 00010).
type EmailStatus string

const (
	EmailStatusNotRequested EmailStatus = "NOT_REQUESTED"
	EmailStatusQueued       EmailStatus = "QUEUED"
	EmailStatusSent         EmailStatus = "SENT"
	EmailStatusFailed       EmailStatus = "FAILED"
)

// Valid reports whether s is a known email status.
func (s EmailStatus) Valid() bool {
	switch s {
	case EmailStatusNotRequested, EmailStatusQueued, EmailStatusSent, EmailStatusFailed:
		return true
	}
	return false
}

const (
	maxRenderedTitleLen = 200
	maxRenderedBodyLen  = 4000
	maxEmailIdempotency = 256
)

var htmlTagPattern = regexp.MustCompile(`(?i)<[a-z/][^>]*>`)

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

// Notification is a row from hrd_notifications.
type Notification struct {
	ID                  uuid.UUID
	EventType           EventType
	EventKey            string
	AdvertID            *uuid.UUID
	PackageAssignmentID *uuid.UUID
	CampaignID          *uuid.UUID
	TemplateID          *uuid.UUID
	Title               string
	Body                string
	Payload             []byte
	CreatedAt           time.Time
}

// InboxItem joins a notification with one user's delivery state.
type InboxItem struct {
	Notification Notification
	State        UserNotificationState
}

// EligibleUser is an ACTIVE fan-out recipient candidate.
type EligibleUser struct {
	ID            uuid.UUID
	Email         string
	EmailVerified bool
}

// HasVerifiedEmail reports whether u has a verified, non-blank email address.
func (u EligibleUser) HasVerifiedEmail() bool {
	return u.EmailVerified && strings.TrimSpace(u.Email) != ""
}

// UserNotificationState is a row from hrd_user_notification_states.
type UserNotificationState struct {
	UserID              uuid.UUID
	NotificationID      uuid.UUID
	DeliveredAt         time.Time
	ReadAt              *time.Time
	EmailStatus         EmailStatus
	EmailIdempotencyKey *string
	EmailAttemptCount   int
	EmailLastAttemptAt  *time.Time
	EmailSentAt         *time.Time
	EmailLastError      *string
	CreatedAt           time.Time
	UpdatedAt           time.Time
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

// AdvancedAdvertPublishedEventKey builds the dedup event key for advanced publish.
func AdvancedAdvertPublishedEventKey(advertID, assignmentID uuid.UUID) string {
	return string(TemplateEventTypeAdvancedAdvertPublished) + ":" + advertID.String() + ":" + assignmentID.String()
}

// UrgentAdvertActivatedEventKey builds the dedup event key for urgent activation.
func UrgentAdvertActivatedEventKey(advertID uuid.UUID, activationVersion int) string {
	return string(TemplateEventTypeUrgentAdvertActivated) + ":" + advertID.String() + ":" + fmt.Sprintf("%d", activationVersion)
}

// PackageExpiryEventKey builds the dedup event key for package expiry reminders.
func PackageExpiryEventKey(assignmentID uuid.UUID, endsAtUTC time.Time, offset PackageExpiryDayOffset) string {
	return "PACKAGE_EXPIRY:" + assignmentID.String() + ":" + endsAtUTC.UTC().Format(time.RFC3339Nano) + ":" + string(offset)
}

// AdvertNotificationEmailIdempotencyKey is deterministic per notification recipient.
func AdvertNotificationEmailIdempotencyKey(notificationID, userID uuid.UUID) string {
	return truncateEmailIdempotencyKey("adv-notif-email:" + notificationID.String() + ":" + userID.String())
}

// PackageExpiryEmailIdempotencyKey is deterministic per expiry notification recipient.
func PackageExpiryEmailIdempotencyKey(notificationID, userID uuid.UUID) string {
	return truncateEmailIdempotencyKey("pkg-expiry-email:" + notificationID.String() + ":" + userID.String())
}

func truncateEmailIdempotencyKey(key string) string {
	if len(key) <= maxEmailIdempotency {
		return key
	}
	return key[:maxEmailIdempotency]
}

// TemplateVars holds rendered template variables (string values only).
type TemplateVars map[string]string

// AllowlistedTemplateVars returns the variable names permitted for an event type.
func AllowlistedTemplateVars(eventType EventType) map[string]struct{} {
	switch eventType {
	case TemplateEventTypeAdvancedAdvertPublished, TemplateEventTypeUrgentAdvertActivated:
		return map[string]struct{}{
			"advertId": {}, "advertTitle": {}, "packageCode": {}, "packageDisplayName": {},
			"isUrgent": {}, "frontendUrl": {},
		}
	case TemplateEventTypePackageExpiry10Days, TemplateEventTypePackageExpiry3Days:
		return map[string]struct{}{
			"advertId": {}, "advertTitle": {}, "packageCode": {}, "packageDisplayName": {},
			"endsAt": {}, "daysRemaining": {},
			"campaignTitle": {}, "campaignDescription": {}, "campaignCtaLabel": {}, "campaignCtaUrl": {},
			"frontendUrl": {},
		}
	default:
		return map[string]struct{}{}
	}
}

// RenderTemplate renders a text/template using only allowlisted vars.
// Missing keys, HTML, or length violations return a sanitized error.
func RenderTemplate(tmpl string, allowlist map[string]struct{}, vars TemplateVars) (string, error) {
	if strings.TrimSpace(tmpl) == "" {
		return "", fmt.Errorf("template is blank")
	}
	for name := range vars {
		if _, ok := allowlist[name]; !ok {
			return "", fmt.Errorf("template variable %q is not allowlisted", name)
		}
	}
	data := make(map[string]string, len(vars))
	for k, v := range vars {
		data[k] = v
	}
	for _, v := range data {
		if err := rejectHTML(v); err != nil {
			return "", err
		}
	}
	parsed, err := template.New("notification").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("invalid template syntax")
	}
	var buf bytes.Buffer
	if err := parsed.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template render failed")
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		return "", fmt.Errorf("rendered output is blank")
	}
	if err := rejectHTML(out); err != nil {
		return "", err
	}
	return out, nil
}

// RenderTitle renders an in-app title with the title length cap.
func RenderTitle(eventType EventType, tmpl string, vars TemplateVars) (string, error) {
	out, err := RenderTemplate(tmpl, AllowlistedTemplateVars(eventType), vars)
	if err != nil {
		return "", err
	}
	if utf8.RuneCountInString(out) > maxRenderedTitleLen {
		return "", fmt.Errorf("rendered title exceeds limit")
	}
	return out, nil
}

// RenderBody renders an in-app body with the body length cap.
func RenderBody(eventType EventType, tmpl string, vars TemplateVars) (string, error) {
	out, err := RenderTemplate(tmpl, AllowlistedTemplateVars(eventType), vars)
	if err != nil {
		return "", err
	}
	if utf8.RuneCountInString(out) > maxRenderedBodyLen {
		return "", fmt.Errorf("rendered body exceeds limit")
	}
	return out, nil
}

func rejectHTML(s string) error {
	if strings.ContainsAny(s, "<>") || htmlTagPattern.MatchString(s) {
		return fmt.Errorf("html is not allowed in template output")
	}
	if strings.Contains(s, "&lt;") || strings.Contains(s, "&gt;") {
		return fmt.Errorf("html entities are not allowed in template output")
	}
	_ = html.EscapeString(s)
	return nil
}

// CalendarDaysUntil counts whole calendar days from now to endsAt in loc (not 24h durations).
func CalendarDaysUntil(endsAtUTC, nowUTC time.Time, loc *time.Location) int {
	if loc == nil {
		loc = time.UTC
	}
	endLocal := endsAtUTC.In(loc)
	nowLocal := nowUTC.In(loc)
	endDate := time.Date(endLocal.Year(), endLocal.Month(), endLocal.Day(), 0, 0, 0, 0, loc)
	nowDate := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	return int(endDate.Sub(nowDate).Hours() / 24)
}

// PackageExpiryTargetDay returns the local calendar day (midnight loc) that is
// offset calendar days after the local calendar day of nowUTC.
func PackageExpiryTargetDay(nowUTC time.Time, loc *time.Location, offset PackageExpiryDayOffset) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	days := 10
	switch offset {
	case PackageExpiryDayOffset3D:
		days = 3
	}
	local := nowUTC.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return today.AddDate(0, 0, days)
}

// AssignmentEndsOnLocalDay reports whether endsAtUTC falls on targetDay in loc.
func AssignmentEndsOnLocalDay(endsAtUTC, targetDay time.Time, loc *time.Location) bool {
	if loc == nil {
		loc = time.UTC
	}
	endLocal := endsAtUTC.In(loc)
	endDate := time.Date(endLocal.Year(), endLocal.Month(), endLocal.Day(), 0, 0, 0, 0, loc)
	target := targetDay.In(loc)
	targetDate := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, loc)
	return endDate.Equal(targetDate)
}
