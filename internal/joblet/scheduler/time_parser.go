package scheduler

import (
	"fmt"
	"time"
)

// FormatScheduleTime formats a time for display in schedule contexts
func FormatScheduleTime(t time.Time) string {
	return t.Format("2006-01-02T15:04:05Z07:00")
}

// ValidateScheduleTime checks if a scheduled time is reasonable (kept for server-side validation)
func ValidateScheduleTime(scheduledTime time.Time) error {
	now := time.Now()

	// Check if time is in the past (with 1 minute tolerance for clock skew)
	if scheduledTime.Before(now.Add(-1 * time.Minute)) {
		return fmt.Errorf("scheduled time %s is in the past", FormatScheduleTime(scheduledTime))
	}

	// Check if time is too far in the future (1 year limit)
	maxFuture := now.Add(365 * 24 * time.Hour)
	if scheduledTime.After(maxFuture) {
		return fmt.Errorf("scheduled time %s is too far in the future (max 1 year)", FormatScheduleTime(scheduledTime))
	}

	return nil
}
