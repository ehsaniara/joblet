package validation

import (
	"fmt"
	"joblet/pkg/platform"
	"path/filepath"
	"strings"
)

// CommandValidator validates commands and arguments
type CommandValidator struct {
	platform         platform.Platform
	maxCommandLength int
	maxArgLength     int
	maxArgCount      int
	dangerousChars   string
	allowedCommands  map[string]bool // Whitelist of allowed commands
	blockedCommands  map[string]bool // Blacklist of dangerous commands
}

// NewCommandValidator creates a new command validator
func NewCommandValidator(platform platform.Platform) *CommandValidator {
	return &CommandValidator{
		platform:         platform,
		maxCommandLength: 1024,
		maxArgLength:     4096,
		maxArgCount:      100,
		dangerousChars:   ";&|`$(){}[]<>",
		blockedCommands: map[string]bool{
			"rm":        true,
			"shutdown":  true,
			"reboot":    true,
			"poweroff":  true,
			"halt":      true,
			"init":      true,
			"systemctl": true, // Could be used to stop system services
		},
		allowedCommands: map[string]bool{
			// Common safe commands
			"echo":     true,
			"sh":       true,
			"ping":     true,
			"curl":     true,
			"wget":     true,
			"hostname": true,
			"sleep":    true,
			"date":     true,
			"pwd":      true,
			"ps":       true,
			"bash":     true,
			"env":      true,
			"mount":    true,
			"ls":       true,
			"cat":      true,
			"grep":     true,
			"find":     true,
			"python":   true,
			"python3":  true,
			"node":     true,
			"java":     true,
			"go":       true,
		},
	}
}

// Validate validates a command and its arguments
func (cv *CommandValidator) Validate(command string, args []string) error {
	// Validate command
	if err := cv.validateCommand(command); err != nil {
		return err
	}

	// Validate arguments
	if err := cv.validateArguments(args); err != nil {
		return err
	}

	// Validate combined command line doesn't create injection
	if err := cv.validateCombined(command, args); err != nil {
		return err
	}

	return nil
}

// validateCommand validates just the command
func (cv *CommandValidator) validateCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	if len(command) > cv.maxCommandLength {
		return fmt.Errorf("command too long (max %d characters)", cv.maxCommandLength)
	}

	// Check for dangerous characters
	for _, char := range cv.dangerousChars {
		if strings.ContainsRune(command, char) {
			return fmt.Errorf("command contains dangerous character: %c", char)
		}
	}

	// Extract base command name for checking
	baseCommand := filepath.Base(command)

	// Check blocklist
	if cv.blockedCommands[baseCommand] {
		return fmt.Errorf("command '%s' is blocked for security reasons", baseCommand)
	}

	// If whitelist is enabled, check it
	if len(cv.allowedCommands) > 0 && !cv.allowedCommands[baseCommand] {
		// Check if it's an absolute path to an allowed command
		if !filepath.IsAbs(command) {
			return fmt.Errorf("command '%s' is not in the allowed list", baseCommand)
		}
	}

	// Validate path traversal attempts
	if strings.Contains(command, "..") {
		return fmt.Errorf("command contains path traversal")
	}

	return nil
}

// validateArguments validates command arguments
func (cv *CommandValidator) validateArguments(args []string) error {
	if len(args) > cv.maxArgCount {
		return fmt.Errorf("too many arguments (max %d)", cv.maxArgCount)
	}

	totalArgLength := 0
	for i, arg := range args {
		// Check for null bytes
		if strings.Contains(arg, "\x00") {
			return fmt.Errorf("args[%d] %s", i, "argument contains null bytes")
		}

		// Check individual arg length
		if len(arg) > cv.maxArgLength {
			return fmt.Errorf(fmt.Sprintf("args[%d]", i),
				fmt.Sprintf("argument too long (max %d characters)", cv.maxArgLength))
		}

		totalArgLength += len(arg)

		// Check for shell injection attempts in args
		if cv.looksLikeShellInjection(arg) {
			return fmt.Errorf(fmt.Sprintf("args[%d]", i),
				"argument appears to contain shell injection")
		}
	}

	// Check total size
	if totalArgLength > cv.maxArgLength*10 {
		return fmt.Errorf("total argument size too large")
	}

	return nil
}

// validateCombined validates the complete command line
func (cv *CommandValidator) validateCombined(command string, args []string) error {
	// Build the full command line
	fullCmd := command
	for _, arg := range args {
		fullCmd += " " + arg
	}

	// Check for common injection patterns
	dangerousPatterns := []string{
		"&&",     // Command chaining
		"||",     // Command chaining
		";",      // Command separator
		"|",      // Pipe (could be legitimate, but check context)
		"$()",    // Command substitution
		"``",     // Command substitution
		"${}",    // Variable expansion
		">/dev/", // Redirecting to devices
		"</dev/", // Reading from devices
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(fullCmd, pattern) {
			return fmt.Errorf("command line contains dangerous pattern: %s", pattern)
		}
	}

	return nil
}

// looksLikeShellInjection checks if an argument looks like shell injection
func (cv *CommandValidator) looksLikeShellInjection(arg string) bool {
	// Count suspicious characters
	suspiciousCount := 0
	suspiciousChars := ";&|`$()<>{}"

	for _, char := range suspiciousChars {
		suspiciousCount += strings.Count(arg, string(char))
	}

	// If more than 2 suspicious characters, it might be injection
	return suspiciousCount > 2
}

// ResolveCommand resolves a command to its full path
func (cv *CommandValidator) ResolveCommand(command string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	// If already absolute, just verify it exists
	if filepath.IsAbs(command) {
		if _, err := cv.platform.Stat(command); err != nil {
			return "", fmt.Errorf("command not found: %s", command)
		}
		return command, nil
	}

	// Try to resolve using PATH
	if resolved, err := cv.platform.LookPath(command); err == nil {
		return resolved, nil
	}

	// Try common locations
	commonPaths := []string{
		filepath.Join("/bin", command),
		filepath.Join("/usr/bin", command),
		filepath.Join("/usr/local/bin", command),
		filepath.Join("/sbin", command),
		filepath.Join("/usr/sbin", command),
	}

	for _, path := range commonPaths {
		if _, err := cv.platform.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("command '%s' not found in PATH or common locations", command)
}

// SetWhitelist sets the allowed commands whitelist
func (cv *CommandValidator) SetWhitelist(commands []string) {
	cv.allowedCommands = make(map[string]bool)
	for _, cmd := range commands {
		cv.allowedCommands[cmd] = true
	}
}

// SetBlacklist sets the blocked commands blacklist
func (cv *CommandValidator) SetBlacklist(commands []string) {
	cv.blockedCommands = make(map[string]bool)
	for _, cmd := range commands {
		cv.blockedCommands[cmd] = true
	}
}
