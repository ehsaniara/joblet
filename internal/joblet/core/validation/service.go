package validation

import (
	"fmt"
	"joblet/internal/joblet/domain"
)

type Service struct {
	commandValidator  *CommandValidator
	scheduleValidator *ScheduleValidator
	resourceValidator *ResourceValidator
}

func NewService(commandValidator *CommandValidator, scheduleValidator *ScheduleValidator, resourceValidator *ResourceValidator) *Service {
	return &Service{
		commandValidator:  commandValidator,
		scheduleValidator: scheduleValidator,
		resourceValidator: resourceValidator,
	}
}

func (v *Service) ResourceValidator() *ResourceValidator {
	return v.resourceValidator
}

func (v *Service) ValidateJobRequest(command string, args []string, schedule string, limits domain.ResourceLimits) error {
	//if err := v.commandValidator.Validate(command, args); err != nil {
	//	return fmt.Errorf("command validation: %w", err)
	//}

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
