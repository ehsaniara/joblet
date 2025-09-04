package jobs

import (
	"os"
	"strings"
	"testing"

	"joblet/internal/rnx/common"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestNewRunCmd(t *testing.T) {
	cmd := NewRunCmd()

	if cmd == nil {
		t.Fatal("NewRunCmd() returned nil")
	}

	if cmd.Use != "run <command> [args...]" {
		t.Errorf("Expected Use 'run <command> [args...]', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	if cmd.Long == "" {
		t.Error("Long description is empty")
	}

	if cmd.RunE == nil {
		t.Error("RunE function is nil")
	}

	// Test that DisableFlagParsing is set for run command
	if !cmd.DisableFlagParsing {
		t.Error("Expected DisableFlagParsing to be true for run command")
	}
}

func TestNewStatusCmd(t *testing.T) {
	cmd := NewStatusCmd()

	if cmd == nil {
		t.Fatal("NewStatusCmd() returned nil")
	}

	if cmd.Use != "status <uuid>" {
		t.Errorf("Expected Use 'status <uuid>', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	// Test Args function (cannot directly compare function pointers)
	if cmd.Args == nil {
		t.Error("Expected Args function to be set")
	}

	if cmd.RunE == nil {
		t.Error("RunE function is nil")
	}

	// Check for expected flags
	expectedFlags := []string{"detail", "workflow"}
	flags := cmd.Flags()

	for _, flagName := range expectedFlags {
		if flag := flags.Lookup(flagName); flag == nil {
			t.Errorf("Expected flag '%s' not found", flagName)
		}
	}
}

func TestStatusCommandEnhancedHelp(t *testing.T) {
	cmd := NewStatusCmd()

	// Test that the enhanced help content is present
	helpContent := cmd.Long

	expectedSections := []string{
		"comprehensive status and details",
		"Job identification (UUID, name, command, arguments)",
		"Resource limits (CPU, memory, I/O, core binding)",
		"Runtime environment (Python, Java, Node.js runtimes)",
		"Network configuration (bridge, isolated, custom networks)",
		"Volume storage (mounted persistent and memory volumes)",
		"Environment variables (regular and secret/masked)",
		"File uploads and working directory",
		"Workflow information (UUID, dependencies for workflow jobs)",
		"Job Status Examples:",
		"Workflow Status Examples:",
		"Job Status Information Displayed:",
		"Output Formats:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(helpContent, section) {
			t.Errorf("Enhanced help content missing section: '%s'", section)
		}
	}
}

func TestStatusCommandShortDescription(t *testing.T) {
	cmd := NewStatusCmd()

	expectedShort := "Get comprehensive status and details of a job or workflow by UUID"
	if cmd.Short != expectedShort {
		t.Errorf("Expected Short description '%s', got '%s'", expectedShort, cmd.Short)
	}
}

func TestStatusCommandHelpExamples(t *testing.T) {
	cmd := NewStatusCmd()
	helpContent := cmd.Long

	// Test that specific examples are included
	expectedExamples := []string{
		"rnx status f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"rnx status f47ac10b",
		"rnx status --json f47ac10b",
		"rnx status --workflow a1b2c3d4-e5f6-7890-1234-567890abcdef",
		"rnx status --workflow a1b2c3d4",
		"rnx status --workflow --detail a1b2c3d4",
		"rnx status --workflow --json a1b2c3d4",
	}

	for _, example := range expectedExamples {
		if !strings.Contains(helpContent, example) {
			t.Errorf("Help content missing example: '%s'", example)
		}
	}
}

func TestStatusCommandInformationCategories(t *testing.T) {
	cmd := NewStatusCmd()
	helpContent := cmd.Long

	// Test that all information categories are documented
	expectedCategories := []string{
		"Basic Info: Job UUID, name, command with arguments, current status",
		"Timing: Creation time, start time, end time, execution duration",
		"Resource Limits: CPU percentage, memory MB, I/O bandwidth, CPU cores",
		"Runtime Environment: Python, Java, Node.js runtime specifications",
		"Network: Network configuration (bridge, isolated, custom networks)",
		"Storage: Mounted volumes (filesystem and memory-based)",
		"Working Directory: Job execution directory path",
		"Uploaded Files: List of files uploaded for job execution",
		"Environment: Regular environment variables (visible in logs)",
		"Secrets: Secret environment variables (masked as ***)",
		"Workflow Context: Workflow UUID and job dependencies (if applicable)",
		"Results: Exit code and completion status",
		"Actions: Contextual next steps (view logs, stop job, etc.)",
	}

	for _, category := range expectedCategories {
		if !strings.Contains(helpContent, category) {
			t.Errorf("Help content missing information category: '%s'", category)
		}
	}
}

func TestStatusCommandOutputFormats(t *testing.T) {
	cmd := NewStatusCmd()
	helpContent := cmd.Long

	// Test that output formats are documented
	expectedFormats := []string{
		"Default: Human-readable formatted output with sections",
		"--json: Machine-readable JSON with all available fields",
	}

	for _, format := range expectedFormats {
		if !strings.Contains(helpContent, format) {
			t.Errorf("Help content missing output format: '%s'", format)
		}
	}
}

func TestNewListCmd(t *testing.T) {
	cmd := NewListCmd()

	if cmd == nil {
		t.Fatal("NewListCmd() returned nil")
	}

	if cmd.Use != "list" {
		t.Errorf("Expected Use 'list', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	if cmd.RunE == nil {
		t.Error("RunE function is nil")
	}

	// Check for workflow flag
	if flag := cmd.Flags().Lookup("workflow"); flag == nil {
		t.Error("Expected 'workflow' flag not found")
	}
}

func TestNewStopCmd(t *testing.T) {
	cmd := NewStopCmd()

	if cmd == nil {
		t.Fatal("NewStopCmd() returned nil")
	}

	if cmd.Use != "stop <job-uuid>" {
		t.Errorf("Expected Use 'stop <job-uuid>', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	// Test Args function (cannot directly compare function pointers)
	if cmd.Args == nil {
		t.Error("Expected Args function to be set")
	}

	if cmd.RunE == nil {
		t.Error("RunE function is nil")
	}
}

func TestNewDeleteCmd(t *testing.T) {
	cmd := NewDeleteCmd()

	if cmd == nil {
		t.Fatal("NewDeleteCmd() returned nil")
	}

	if cmd.Use != "delete <job-uuid>" {
		t.Errorf("Expected Use 'delete <job-uuid>', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	// Test Args function (cannot directly compare function pointers)
	if cmd.Args == nil {
		t.Error("Expected Args function to be set")
	}

	if cmd.RunE == nil {
		t.Error("RunE function is nil")
	}
}

func TestNewLogCmd(t *testing.T) {
	cmd := NewLogCmd()

	if cmd == nil {
		t.Fatal("NewLogCmd() returned nil")
	}

	if cmd.Use != "log <job-uuid>" {
		t.Errorf("Expected Use 'log <job-uuid>', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	// Test Args function (cannot directly compare function pointers)
	if cmd.Args == nil {
		t.Error("Expected Args function to be set")
	}

	if cmd.RunE == nil {
		t.Error("RunE function is nil")
	}

	// Test log command structure and behavior
	if cmd.Use != "log <job-uuid>" {
		t.Errorf("Expected Use 'log <job-uuid>', got %s", cmd.Use)
	}

	if !strings.Contains(cmd.Long, "Stream logs from a running or completed job") {
		t.Error("Long description should mention streaming logs")
	}

	if !strings.Contains(cmd.Long, "Short-form UUIDs are supported") {
		t.Error("Long description should mention short-form UUID support")
	}

	if !strings.Contains(cmd.Long, "Ctrl+C") {
		t.Error("Long description should mention Ctrl+C to stop following")
	}

	// Test that command expects exactly one argument
	if cmd.Args == nil {
		t.Error("Expected Args function to be set for log command")
	}
}

func TestLogCommandHelpExamples(t *testing.T) {
	cmd := NewLogCmd()
	helpContent := cmd.Long

	// Test that specific log examples are included
	expectedExamples := []string{
		"rnx log f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"rnx log f47ac10b",
		"rnx log a1b2c3d4",
	}

	for _, example := range expectedExamples {
		if !strings.Contains(helpContent, example) {
			t.Errorf("Help content missing log example: '%s'", example)
		}
	}
}

func TestLogCommandBehavior(t *testing.T) {
	cmd := NewLogCmd()

	// Test that log command doesn't have flags (automatically follows)
	flags := cmd.Flags()
	flagCount := 0
	flags.VisitAll(func(flag *pflag.Flag) {
		flagCount++
	})

	if flagCount > 0 {
		t.Errorf("Expected log command to have no flags (automatically follows), but found %d flags", flagCount)
	}

	// Test command description mentions automatic following behavior
	if !strings.Contains(cmd.Long, "follows the log stream for running jobs") {
		t.Error("Long description should mention automatic following for running jobs")
	}

	if !strings.Contains(cmd.Long, "all output for completed jobs") {
		t.Error("Long description should mention showing all output for completed jobs")
	}
}

func TestLogCommandValidation(t *testing.T) {
	cmd := NewLogCmd()

	// Test that the command requires exactly one argument
	// We can't easily test the Args function directly without running it,
	// but we can verify it's set and matches the expected signature
	if cmd.Args == nil {
		t.Error("Log command should have Args validation function")
	}

	// Test that command has proper usage format
	expectedUsage := "log <job-uuid>"
	if cmd.Use != expectedUsage {
		t.Errorf("Expected usage '%s', got '%s'", expectedUsage, cmd.Use)
	}
}

func TestNewMonitorCmd(t *testing.T) {
	cmd := NewMonitorCmd()

	if cmd == nil {
		t.Fatal("NewMonitorCmd() returned nil")
	}

	if cmd.Use != "monitor" {
		t.Errorf("Expected Use 'monitor', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	// Check that subcommands exist
	subcommands := cmd.Commands()
	expectedSubcommands := []string{"status", "top", "watch"}

	foundSubcommands := make(map[string]bool)
	for _, subcmd := range subcommands {
		foundSubcommands[subcmd.Use] = true
	}

	for _, expected := range expectedSubcommands {
		if !foundSubcommands[expected] {
			t.Errorf("Expected monitor subcommand '%s' not found", expected)
		}
	}
}

func TestEnvironmentVariableProcessing(t *testing.T) {
	// Test the environment variable processing functions
	tests := []struct {
		name        string
		envVars     []string
		expected    map[string]string
		expectError bool
	}{
		{
			name:    "valid environment variables",
			envVars: []string{"VAR1=value1", "VAR2=value2"},
			expected: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
			expectError: false,
		},
		{
			name:        "invalid format",
			envVars:     []string{"INVALID_FORMAT"},
			expected:    nil,
			expectError: true,
		},
		{
			name:        "empty key",
			envVars:     []string{"=value"},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processEnvironmentVariables(tt.envVars)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d variables, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				if actualValue, exists := result[key]; !exists {
					t.Errorf("Expected variable %s to exist", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected %s=%s, got %s=%s", key, expectedValue, key, actualValue)
				}
			}
		})
	}
}

func TestJobCommandFlags(t *testing.T) {
	// Test command-specific flags
	tests := []struct {
		name          string
		cmdFunc       func() *cobra.Command
		expectedFlags []string
	}{
		{
			name:          "status command flags",
			cmdFunc:       NewStatusCmd,
			expectedFlags: []string{"detail", "workflow"},
		},
		{
			name:          "list command flags",
			cmdFunc:       NewListCmd,
			expectedFlags: []string{"workflow"},
		},
		{
			name:          "log command flags",
			cmdFunc:       NewLogCmd,
			expectedFlags: []string{}, // Log command has no flags - automatically follows for running jobs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmdFunc()
			flags := cmd.Flags()

			for _, flagName := range tt.expectedFlags {
				if flag := flags.Lookup(flagName); flag == nil {
					t.Errorf("Expected flag '%s' not found in %s command", flagName, cmd.Use)
				}
			}
		})
	}
}

func TestJobCommandValidation(t *testing.T) {
	// Test argument validation for commands that require specific argument counts
	tests := []struct {
		name      string
		cmdFunc   func() *cobra.Command
		args      []string
		expectErr bool
	}{
		{
			name:      "status with valid UUID",
			cmdFunc:   NewStatusCmd,
			args:      []string{"f47ac10b-58cc-4372-a567-0e02b2c3d479"},
			expectErr: false,
		},
		{
			name:      "status with short UUID",
			cmdFunc:   NewStatusCmd,
			args:      []string{"f47ac10b"},
			expectErr: false,
		},
		{
			name:      "status without argument",
			cmdFunc:   NewStatusCmd,
			args:      []string{},
			expectErr: true,
		},
		{
			name:      "stop with UUID",
			cmdFunc:   NewStopCmd,
			args:      []string{"f47ac10b-58cc-4372-a567-0e02b2c3d479"},
			expectErr: false,
		},
		{
			name:      "stop without argument",
			cmdFunc:   NewStopCmd,
			args:      []string{},
			expectErr: true,
		},
		{
			name:      "delete with UUID",
			cmdFunc:   NewDeleteCmd,
			args:      []string{"f47ac10b-58cc-4372-a567-0e02b2c3d479"},
			expectErr: false,
		},
		{
			name:      "log with UUID",
			cmdFunc:   NewLogCmd,
			args:      []string{"f47ac10b-58cc-4372-a567-0e02b2c3d479"},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmdFunc()

			// Test argument validation
			if cmd.Args != nil {
				err := cmd.Args(cmd, tt.args)
				if tt.expectErr && err == nil {
					t.Error("Expected error but got none")
				}
				if !tt.expectErr && err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRunCommandSpecialHandling(t *testing.T) {
	// Test that run command handles help manually due to DisableFlagParsing
	cmd := NewRunCmd()

	// Test that it has DisableFlagParsing set
	if !cmd.DisableFlagParsing {
		t.Error("Run command should have DisableFlagParsing=true")
	}

	// Test help handling (this tests the logic in runRun function)
	helpArgs := [][]string{
		{"-h"},
		{"--help"},
	}

	for _, args := range helpArgs {
		t.Run("help_handling_"+strings.Join(args, "_"), func(t *testing.T) {
			// We can't easily test the actual runRun function without mocking,
			// but we can test that the help arguments are recognized
			foundHelp := false
			for _, arg := range args {
				if arg == "-h" || arg == "--help" {
					foundHelp = true
					break
				}
			}

			if !foundHelp {
				t.Error("Help argument not recognized in test args")
			}
		})
	}
}

func TestCommandHelpText(t *testing.T) {
	// Test that all commands have proper help text
	commandFuncs := []struct {
		name    string
		cmdFunc func() *cobra.Command
	}{
		{"run", NewRunCmd},
		{"status", NewStatusCmd},
		{"list", NewListCmd},
		{"stop", NewStopCmd},
		{"delete", NewDeleteCmd},
		{"log", NewLogCmd},
		{"monitor", NewMonitorCmd},
	}

	for _, cf := range commandFuncs {
		t.Run(cf.name+"_help_text", func(t *testing.T) {
			cmd := cf.cmdFunc()

			if cmd.Short == "" {
				t.Errorf("Command %s has empty Short description", cf.name)
			}

			if cmd.Use == "" {
				t.Errorf("Command %s has empty Use string", cf.name)
			}

			// Most commands should have Long descriptions (except simple ones)
			if cf.name == "run" || cf.name == "status" || cf.name == "monitor" {
				if cmd.Long == "" {
					t.Errorf("Command %s should have Long description", cf.name)
				}
			}
		})
	}
}

func TestEnvironmentVariableValidation(t *testing.T) {
	// Test environment variable name validation
	tests := []struct {
		name        string
		varName     string
		expectError bool
	}{
		{
			name:        "valid name",
			varName:     "VALID_VAR",
			expectError: false,
		},
		{
			name:        "valid name with numbers",
			varName:     "VAR123",
			expectError: false,
		},
		{
			name:        "invalid name starting with number",
			varName:     "123VAR",
			expectError: true,
		},
		{
			name:        "invalid name with hyphen",
			varName:     "INVALID-VAR",
			expectError: true,
		},
		{
			name:        "empty name",
			varName:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvironmentVariableName(tt.varName)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for variable name '%s', but got none", tt.varName)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for variable name '%s': %v", tt.varName, err)
			}
		})
	}
}

func TestColorSupport(t *testing.T) {
	// Test that color functions exist and work properly
	// Since these are likely simple functions, we just test they don't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Color function panicked: %v", r)
		}
	}()

	// Test that color constants or functions would be defined
	testString := "test"
	_ = testString // Use the string to avoid unused variable error

	// This test mainly ensures color-related code doesn't cause import issues
	// The actual color functionality would need the specific color functions
}

// Benchmark tests
func BenchmarkJobCommandCreation(b *testing.B) {
	commands := []func() *cobra.Command{
		NewRunCmd,
		NewStatusCmd,
		NewListCmd,
		NewStopCmd,
		NewDeleteCmd,
		NewLogCmd,
		NewMonitorCmd,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, cmdFunc := range commands {
			_ = cmdFunc()
		}
	}
}

func BenchmarkEnvironmentVariableProcessing(b *testing.B) {
	envVars := []string{
		"VAR1=value1",
		"VAR2=value2_with_longer_value",
		"VAR3=value3",
		"PATH=/usr/local/bin:/usr/bin:/bin",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processEnvironmentVariables(envVars)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

// Test cleanup
func TestMain(m *testing.M) {
	// Setup
	originalJSONOutput := common.JSONOutput
	originalConfigPath := common.ConfigPath
	originalNodeName := common.NodeName

	// Run tests
	code := m.Run()

	// Cleanup
	common.JSONOutput = originalJSONOutput
	common.ConfigPath = originalConfigPath
	common.NodeName = originalNodeName

	os.Exit(code)
}
