package validation

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ScheduleValidator validates job scheduling parameters
type ScheduleValidator struct {
	minAdvanceTime   time.Duration // Minimum time in the future
	maxAdvanceTime   time.Duration // Maximum time in the future
	granularity      time.Duration // Minimum scheduling granularity
	maxScheduledJobs int           // Maximum number of scheduled jobs
	currentJobCount  func() int    // Function to get current scheduled job count
}

// NewScheduleValidator creates a new schedule validator
func NewScheduleValidator() *ScheduleValidator {
	return &ScheduleValidator{
		minAdvanceTime:   1 * time.Minute,      // Jobs must be at least 1 minute in future
		maxAdvanceTime:   365 * 24 * time.Hour, // Maximum 1 year in advance
		granularity:      1 * time.Second,      // Second-level precision
		maxScheduledJobs: 1000,                 // Default max scheduled jobs
	}
}

// Validate validates a schedule specification
func (sv *ScheduleValidator) Validate(scheduleSpec string) error {
	// Parse the schedule time
	scheduledTime, err := sv.parseScheduleTime(scheduleSpec)
	if err != nil {
		return fmt.Errorf("invalid schedule format: %v", err)
	}

	// Validate the parsed time
	return sv.ValidateTime(scheduledTime)
}

// ValidateTime validates a scheduled time
func (sv *ScheduleValidator) ValidateTime(scheduledTime time.Time) error {
	now := time.Now()

	// Check if time is in the past (with small grace period for clock skew)
	if scheduledTime.Before(now.Add(-30 * time.Second)) {
		return fmt.Errorf("scheduled time is in the past: %s", scheduledTime.Format(time.RFC3339))
	}

	// Check minimum advance time
	if scheduledTime.Before(now.Add(sv.minAdvanceTime)) {
		return fmt.Errorf("scheduled time must be at least %v in the future", sv.minAdvanceTime)
	}

	// Check maximum advance time
	if scheduledTime.After(now.Add(sv.maxAdvanceTime)) {
		return fmt.Errorf("scheduled time too far in future (max %v)", sv.maxAdvanceTime)
	}

	// Check if we've hit the scheduled job limit
	if sv.currentJobCount != nil {
		currentCount := sv.currentJobCount()
		if currentCount >= sv.maxScheduledJobs {
			return fmt.Errorf("too many scheduled jobs (max %d)", sv.maxScheduledJobs)
		}
	}

	return nil
}

// parseScheduleTime parses various schedule time formats
func (sv *ScheduleValidator) parseScheduleTime(scheduleSpec string) (time.Time, error) {
	// Try multiple formats in order of preference
	formats := []string{
		time.RFC3339,                // "2006-01-02T15:04:05Z07:00"
		time.RFC3339Nano,            // "2006-01-02T15:04:05.999999999Z07:00"
		"2006-01-02T15:04:05",       // Simple ISO format without timezone
		"2006-01-02 15:04:05",       // Space-separated
		"2006-01-02T15:04:05Z",      // UTC with Z
		"2006-01-02T15:04:05-07:00", // With timezone offset
	}

	var parseErr error
	for _, format := range formats {
		if t, err := time.Parse(format, scheduleSpec); err == nil {
			// If no timezone info, assume local time
			if format == "2006-01-02T15:04:05" || format == "2006-01-02 15:04:05" {
				t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
			}
			return t, nil
		} else {
			parseErr = err
		}
	}

	// Try parsing as duration from now (e.g., "+5m", "+1h")
	if strings.HasPrefix(scheduleSpec, "+") {
		if duration, err := time.ParseDuration(scheduleSpec[1:]); err == nil {
			return time.Now().Add(duration), nil
		}
	}

	// Try parsing as Unix timestamp
	if timestamp, err := strconv.ParseInt(scheduleSpec, 10, 64); err == nil {
		// Assume seconds if reasonable, otherwise milliseconds
		if timestamp < 10000000000 {
			return time.Unix(timestamp, 0), nil
		}
		return time.Unix(timestamp/1000, (timestamp%1000)*1000000), nil
	}

	return time.Time{}, fmt.Errorf("unable to parse schedule time '%s': %v", scheduleSpec, parseErr)
}

// ValidateCronExpression validates a cron expression (for future cron support)
func (sv *ScheduleValidator) ValidateCronExpression(cronExpr string) error {
	// Basic validation for cron expressions
	// Format: "minute hour day month weekday"
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 && len(fields) != 6 { // 5 for standard, 6 for with seconds
		return fmt.Errorf("invalid cron expression: expected 5 or 6 fields")
	}

	// Validate each field (simplified)
	for i, field := range fields {
		if field != "*" && field != "?" {
			// Check if it's a number or range
			if !sv.isValidCronField(field, i) {
				return fmt.Errorf("invalid cron field at position %d: %s", i, field)
			}
		}
	}

	return nil
}

// isValidCronField validates a single cron field
func (sv *ScheduleValidator) isValidCronField(field string, position int) bool {
	// Define valid ranges for each position
	ranges := []struct{ min, max int }{
		{0, 59}, // minute
		{0, 23}, // hour
		{1, 31}, // day of month
		{1, 12}, // month
		{0, 7},  // day of week (0 and 7 are Sunday)
		{0, 59}, // second (if 6 fields)
	}

	if position >= len(ranges) {
		return false
	}

	// Handle ranges (e.g., "10-20")
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) != 2 {
			return false
		}
		// Validate both parts are in range
		// (simplified - real implementation would parse and check)
		return true
	}

	// Handle lists (e.g., "1,3,5")
	if strings.Contains(field, ",") {
		// Validate each item in list
		// (simplified)
		return true
	}

	// Handle step values (e.g., "*/5")
	if strings.Contains(field, "/") {
		// Validate step syntax
		// (simplified)
		return true
	}

	// Simple number
	if num, err := strconv.Atoi(field); err == nil {
		r := ranges[position]
		return num >= r.min && num <= r.max
	}

	return false
}

// SetJobCountFunc sets the function to get current scheduled job count
func (sv *ScheduleValidator) SetJobCountFunc(f func() int) {
	sv.currentJobCount = f
}

// SetLimits configures validation limits
func (sv *ScheduleValidator) SetLimits(minAdvance, maxAdvance time.Duration, maxJobs int) {
	sv.minAdvanceTime = minAdvance
	sv.maxAdvanceTime = maxAdvance
	sv.maxScheduledJobs = maxJobs
}
