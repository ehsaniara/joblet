package validation

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Common validation functions consolidated from multiple validators
// This reduces duplication across NetworkValidator, CommandValidator, etc.

var (
	// Common regex patterns for validation
	validNameRegex   = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]*$`)
	validEnvVarRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// ValidateName validates any resource name (network, volume, job, etc.)
func ValidateName(name string, resourceType string, maxLength int) error {
	if name == "" {
		return fmt.Errorf("%s name cannot be empty", resourceType)
	}

	if len(name) > maxLength {
		return fmt.Errorf("%s name too long (max %d chars)", resourceType, maxLength)
	}

	if !validNameRegex.MatchString(name) {
		return fmt.Errorf("invalid %s name: must start with letter or underscore, contain only alphanumeric, dash, and underscore", resourceType)
	}

	return nil
}

// ValidateEnvironmentVariable validates an environment variable name and value
func ValidateEnvironmentVariable(key, value string) error {
	if !validEnvVarRegex.MatchString(key) {
		return fmt.Errorf("invalid environment variable name '%s': must start with letter or underscore and contain only letters, numbers, and underscores", key)
	}

	// Check value length (32KB limit)
	if len(value) > 32768 {
		return fmt.Errorf("environment variable '%s' value too long (%d bytes, max 32768)", key, len(value))
	}

	return nil
}

// ContainsDangerousPatterns checks for potentially dangerous patterns in strings
func ContainsDangerousPatterns(value string) bool {
	dangerousPatterns := []string{
		"$(", "`", // Command substitution
		"rm -rf", "format C:", "del /f", // Dangerous commands
		"../",                   // Path traversal
		"/etc/shadow", "passwd", // System files
	}

	lowerValue := strings.ToLower(value)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerValue, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// IsReservedName checks if a name conflicts with system reserved names
func IsReservedName(name string, reservedNames map[string]bool) bool {
	return reservedNames[name]
}

// Validator provides common validation logic for all resources
type Validator struct {
	// Can be extended with common fields if needed
}

// NewValidator creates a new base validator
func NewValidator() *Validator {
	return &Validator{}
}

// ValidatePathWithinBase ensures that a target path stays within the base directory.
// Returns the validated absolute path if valid, or an error if the path escapes.
// This prevents path traversal attacks (e.g., "../../../etc/passwd").
func ValidatePathWithinBase(basePath, targetPath string) (string, error) {
	if basePath == "" {
		return "", fmt.Errorf("base path cannot be empty")
	}

	// Reject absolute paths immediately - they should always be relative to base
	if filepath.IsAbs(targetPath) {
		return "", fmt.Errorf("path escapes base directory: %s", targetPath)
	}

	// Join and clean the paths
	fullPath := filepath.Join(basePath, targetPath)

	// Resolve to absolute paths for accurate comparison
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("invalid base path: %w", err)
	}

	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid target path: %w", err)
	}

	// Ensure the full path is within the base directory
	// We append filepath.Separator to prevent "/workspace-other" matching "/workspace"
	if absFull != absBase && !strings.HasPrefix(absFull, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes base directory: %s", targetPath)
	}

	return absFull, nil
}
