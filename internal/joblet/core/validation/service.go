package validation

import (
	"fmt"
	"joblet/internal/joblet/domain"
)

// Service provides comprehensive job request validation.
// Coordinates command, schedule, and resource validators to ensure
// all job parameters are valid before execution.
type Service struct {
	commandValidator  *CommandValidator
	scheduleValidator *ScheduleValidator
	resourceValidator *ResourceValidator
}

// NewService creates a new validation service with all validators.
// Initializes the service with command, schedule, and resource validators
// for comprehensive job request validation.
func NewService(commandValidator *CommandValidator, scheduleValidator *ScheduleValidator, resourceValidator *ResourceValidator) *Service {
	return &Service{
		commandValidator:  commandValidator,
		scheduleValidator: scheduleValidator,
		resourceValidator: resourceValidator,
	}
}

// ResourceValidator returns the resource validator instance.
// Exposes the resource validator for use by job builder
// for resource limit calculations and validation.
func (v *Service) ResourceValidator() *ResourceValidator {
	return v.resourceValidator
}

// ValidateJobRequest validates all aspects of a job request.
// Checks schedule format (if provided) and resource limits,
// returning detailed error if any validation fails.
func (v *Service) ValidateJobRequest(command string, args []string, schedule string, limits domain.ResourceLimits) error {
	if schedule != "" {
		if err := v.scheduleValidator.Validate(schedule); err != nil {
			return fmt.Errorf("schedule validation: %w", err)
		}
	}

	if err := v.resourceValidator.Validate(limits); err != nil {
		return fmt.Errorf("resource validation: %w", err)
	}

	return nil
}
