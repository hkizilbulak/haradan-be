package jobdef

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// cronParser validates 6-field cron expressions (with seconds) in Europe/Istanbul.
var cronParser = cron.NewParser(
	cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// ValidateCronExpression reports whether expr is a valid 6-field (seconds)
// cron expression interpretable in Europe/Istanbul.
func ValidateCronExpression(expr string) error {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return fmt.Errorf("cron expression is required")
	}
	schedule, err := cronParser.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	_ = schedule
	return nil
}

// ParseCronSchedule parses a validated 6-field cron expression.
func ParseCronSchedule(expr string) (cron.Schedule, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return nil, fmt.Errorf("cron expression is required")
	}
	return cronParser.Parse(trimmed)
}

// NextRunAt returns the next cron fire time after now for an active definition.
// Inactive definitions and invalid cron expressions yield nil.
func NextRunAt(def JobDefinition, now time.Time) *time.Time {
	if !def.IsActive {
		return nil
	}
	sched, err := ParseCronSchedule(def.CronExpression)
	if err != nil {
		return nil
	}
	next := sched.Next(now.In(Istanbul()))
	if next.IsZero() {
		return nil
	}
	utc := next.UTC()
	return &utc
}
