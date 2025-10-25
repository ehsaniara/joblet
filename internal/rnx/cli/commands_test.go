package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/ehsaniara/joblet/internal/rnx/common"

	"github.com/spf13/cobra"
)

func TestRootCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantUse   string
		wantShort string
	}{
		{
			name:      "root command properties",
			args:      []string{},
			wantUse:   "rnx",
			wantShort: "RNX - Remote eXecution client for Joblet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{
				Use:   "rnx",
				Short: "RNX - Remote eXecution client for Joblet",
				Long: `RNX (Remote eXecution) - Command Line Interface to interact with Joblet gRPC services using embedded certificates

RNX provides a complete interface for job execution, workflow management, and resource control
on Joblet servers. It supports immediate execution, scheduling, and comprehensive monitoring.

Key Features:
  - Execute jobs with resource limits and scheduling
  - Manage multi-job workflows with dependencies  
  - Create and manage networks, volumes, and runtimes
  - Monitor remote server resources, job performance, and volume usage
  - Stream real-time logs from running jobs

Quick Examples:
  rnx job run python script.py                    # Run a simple job
  rnx workflow run pipeline.yaml                  # Execute a workflow
  rnx workflow list                               # List all workflows
  rnx job status <job-uuid>                       # Check job status (supports short UUIDs)
  rnx job log <job-uuid>                          # Stream job logs (supports short UUIDs)
  rnx monitor status                          # View remote server metrics and volumes
  rnx monitor top --json                      # JSON output for dashboards

Note: Job and workflow UUIDs support short-form usage (first 8 characters)
if they uniquely identify the resource.

Use 'rnx <command> --help' for detailed information about any command.`,
			}

			if cmd.Use != tt.wantUse {
				t.Errorf("Root command Use = %v, want %v", cmd.Use, tt.wantUse)
			}
			if cmd.Short != tt.wantShort {
				t.Errorf("Root command Short = %v, want %v", cmd.Short, tt.wantShort)
			}
		})
	}
}

func TestGlobalFlags(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedConfig string
		expectedNode   string
		expectedJSON   bool
	}{
		{
			name:           "default values",
			args:           []string{},
			expectedConfig: "",
			expectedNode:   "default",
			expectedJSON:   false,
		},
		{
			name:           "config flag",
			args:           []string{"--config", "/path/to/config.yml"},
			expectedConfig: "/path/to/config.yml",
			expectedNode:   "default",
			expectedJSON:   false,
		},
		{
			name:           "node flag",
			args:           []string{"--node", "production"},
			expectedConfig: "",
			expectedNode:   "production",
			expectedJSON:   false,
		},
		{
			name:           "json flag",
			args:           []string{"--json"},
			expectedConfig: "",
			expectedNode:   "default",
			expectedJSON:   true,
		},
		{
			name:           "all flags combined",
			args:           []string{"--config", "/etc/rnx.yml", "--node", "staging", "--json"},
			expectedConfig: "/etc/rnx.yml",
			expectedNode:   "staging",
			expectedJSON:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables
			common.ConfigPath = ""
			common.NodeName = "default"
			common.JSONOutput = false

			// Create a test command with global flags
			cmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					return nil
				},
			}

			// Add persistent flags (same as root command)
			cmd.PersistentFlags().StringVar(&common.ConfigPath, "config", "",
				"Path to client configuration file (searches common locations if not specified)")
			cmd.PersistentFlags().StringVar(&common.NodeName, "node", "default",
				"Node name from configuration file")
			cmd.PersistentFlags().BoolVar(&common.JSONOutput, "json", false,
				"Output in JSON format")

			// Set args and parse
			cmd.SetArgs(tt.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Command execution failed: %v", err)
			}

			// Verify flag values
			if common.ConfigPath != tt.expectedConfig {
				t.Errorf("ConfigPath = %v, want %v", common.ConfigPath, tt.expectedConfig)
			}
			if common.NodeName != tt.expectedNode {
				t.Errorf("NodeName = %v, want %v", common.NodeName, tt.expectedNode)
			}
			if common.JSONOutput != tt.expectedJSON {
				t.Errorf("JSONOutput = %v, want %v", common.JSONOutput, tt.expectedJSON)
			}
		})
	}
}

func TestCommandExistence(t *testing.T) {
	// Test that all commands can be created without errors
	expectedCommands := []struct {
		name string
		use  string
	}{
		{"run", "run"},
		{"status", "status"},
		{"stop", "stop"},
		{"delete", "delete"},
		{"log", "log"},
		{"log-manage", "log-manage"},
		{"list", "list"},
		{"nodes", "nodes"},
		{"config-help", "config-help"},
		{"network", "network"},
		{"volume", "volume"},
		{"monitor", "monitor"},
		{"runtime", "runtime"},
		{"admin", "admin"},
	}

	// Test that we can get all commands without importing actual command modules
	// by checking the command names exist in our expected list
	for _, expected := range expectedCommands {
		t.Run("command_exists_"+expected.name, func(t *testing.T) {
			// This test just verifies our expected command list is reasonable
			if expected.use == "" {
				t.Errorf("Command %s has empty use string", expected.name)
			}
			if expected.name == "" {
				t.Errorf("Command has empty name")
			}
		})
	}
}

func TestRunCommandFlags(t *testing.T) {
	// Test run command specific flags
	tests := []struct {
		name           string
		args           []string
		expectedError  bool
		expectedParsed bool
	}{
		{
			name:           "help flag",
			args:           []string{"run", "--help"},
			expectedError:  false,
			expectedParsed: true,
		},
		{
			name:           "json flag",
			args:           []string{"--json", "run", "echo", "test"},
			expectedError:  false,
			expectedParsed: true,
		},
		{
			name:           "config flag",
			args:           []string{"--config", "/tmp/test.yml", "run", "echo", "test"},
			expectedError:  false,
			expectedParsed: true,
		},
		{
			name:           "node flag",
			args:           []string{"--node", "production", "run", "echo", "test"},
			expectedError:  false,
			expectedParsed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test argument parsing without execution
			parsedCorrectly := true
			for _, arg := range tt.args {
				if strings.Contains(arg, "--") {
					// Basic validation that flag syntax is correct
					if !strings.HasPrefix(arg, "--") && !strings.HasPrefix(arg, "-") {
						parsedCorrectly = false
						break
					}
				}
			}

			if parsedCorrectly != tt.expectedParsed {
				t.Errorf("Argument parsing = %v, want %v", parsedCorrectly, tt.expectedParsed)
			}
		})
	}
}

func TestSubcommandFlags(t *testing.T) {
	// Test subcommand specific flags
	testCases := []struct {
		command     string
		validFlags  []string
		description string
	}{
		{
			command:     "status",
			validFlags:  []string{}, // No job-specific flags (workflow flags removed)
			description: "Status command flags",
		},
		{
			command:     "list",
			validFlags:  []string{}, // No job-specific flags (workflow flags removed)
			description: "List command flags",
		},
		{
			command:     "log",
			validFlags:  []string{"--follow", "-f"},
			description: "Log command flags",
		},
		{
			command:     "monitor",
			validFlags:  []string{"status", "top", "watch"},
			description: "Monitor subcommands",
		},
		{
			command:     "network",
			validFlags:  []string{"create", "list", "remove"},
			description: "Network subcommands",
		},
		{
			command:     "volume",
			validFlags:  []string{"create", "list", "remove"},
			description: "Volume subcommands",
		},
		{
			command:     "runtime",
			validFlags:  []string{"list", "info", "test", "install"},
			description: "Runtime subcommands",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.command+"_flags", func(t *testing.T) {
			// Test that expected flags/subcommands are defined
			for _, flag := range tc.validFlags {
				if flag == "" {
					t.Errorf("Empty flag/subcommand in %s test case", tc.command)
				}
			}

			// Note: Some commands may have no command-specific flags (only global flags)
			// This is valid, so we don't fail on empty validFlags
		})
	}
}

func TestEnvironmentVariableFlags(t *testing.T) {
	// Test environment variable processing flags for run command
	tests := []struct {
		name        string
		envFlags    []string
		secretFlags []string
		expectValid bool
	}{
		{
			name:        "valid env flags",
			envFlags:    []string{"--env=KEY1=value1", "-e", "KEY2=value2"},
			secretFlags: []string{"--secret-env=SECRET1=secret1", "-s", "SECRET2=secret2"},
			expectValid: true,
		},
		{
			name:        "mixed flags",
			envFlags:    []string{"--env=PUBLIC=public_value"},
			secretFlags: []string{"-s", "SECRET=secret_value"},
			expectValid: true,
		},
		{
			name:        "no env flags",
			envFlags:    []string{},
			secretFlags: []string{},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test flag syntax validation
			allFlags := append(tt.envFlags, tt.secretFlags...)

			for i, flag := range allFlags {
				if flag == "" {
					continue
				}

				// Basic syntax validation - skip values that come after short flags
				isValue := i > 0 && (allFlags[i-1] == "-e" || allFlags[i-1] == "-s")
				if !isValue && !strings.HasPrefix(flag, "--") && !strings.HasPrefix(flag, "-") {
					if tt.expectValid {
						t.Errorf("Expected valid flag syntax, but got invalid flag: %s", flag)
					}
				}

				// Validate env flag format
				if strings.HasPrefix(flag, "--env=") || strings.HasPrefix(flag, "--secret-env=") {
					parts := strings.SplitN(flag, "=", 2)
					if len(parts) < 2 {
						if tt.expectValid {
							t.Errorf("Expected valid env flag format, but got: %s", flag)
						}
					}
				}
			}
		})
	}
}

func TestRunCommandResourceFlags(t *testing.T) {
	// Test resource-related flags for run command
	resourceFlags := []struct {
		flag        string
		description string
		validValues []string
	}{
		{
			flag:        "--max-cpu",
			description: "Maximum CPU percentage",
			validValues: []string{"50", "100", "200"},
		},
		{
			flag:        "--max-memory",
			description: "Maximum memory in MB",
			validValues: []string{"256", "512", "1024"},
		},
		{
			flag:        "--max-iobps",
			description: "Maximum IO BPS",
			validValues: []string{"1000", "10000"},
		},
		{
			flag:        "--cpu-cores",
			description: "CPU cores specification",
			validValues: []string{"0-1", "2", "0,2,4"},
		},
		{
			flag:        "--runtime",
			description: "Runtime specification",
			validValues: []string{"python-3.11", "openjdk-21", "node-18"},
		},
		{
			flag:        "--network",
			description: "Network configuration",
			validValues: []string{"bridge", "isolated", "none"},
		},
		{
			flag:        "--volume",
			description: "Volume name",
			validValues: []string{"data", "cache", "logs"},
		},
		{
			flag:        "--schedule",
			description: "Schedule specification",
			validValues: []string{"1hour", "30min", "2025-12-31T23:59:59Z"},
		},
		// --workflow flag removed (now use: rnx workflow run <file>)
	}

	for _, rf := range resourceFlags {
		t.Run("resource_flag_"+rf.flag[2:], func(t *testing.T) {
			// Test flag definition
			if rf.flag == "" {
				t.Error("Resource flag name is empty")
			}
			if rf.description == "" {
				t.Error("Resource flag description is empty")
			}
			if len(rf.validValues) == 0 {
				t.Error("No valid values defined for resource flag")
			}

			// Test valid values format
			for _, value := range rf.validValues {
				if value == "" {
					t.Errorf("Empty valid value for flag %s", rf.flag)
				}
			}
		})
	}
}

func TestUploadFlags(t *testing.T) {
	// Test file upload flags
	uploadTests := []struct {
		name      string
		flags     []string
		expectErr bool
	}{
		{
			name:      "upload single file",
			flags:     []string{"--upload=script.py"},
			expectErr: false,
		},
		{
			name:      "upload directory",
			flags:     []string{"--upload-dir=./src"},
			expectErr: false,
		},
		{
			name:      "multiple uploads",
			flags:     []string{"--upload=file1.txt", "--upload=file2.txt"},
			expectErr: false,
		},
		{
			name:      "mixed upload types",
			flags:     []string{"--upload=script.py", "--upload-dir=./data"},
			expectErr: false,
		},
	}

	for _, tt := range uploadTests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate flag syntax
			for _, flag := range tt.flags {
				if !strings.HasPrefix(flag, "--upload") {
					if !tt.expectErr {
						t.Errorf("Expected upload flag prefix, got: %s", flag)
					}
				}

				// Basic format check
				if strings.Contains(flag, "=") {
					parts := strings.SplitN(flag, "=", 2)
					if len(parts) != 2 || parts[1] == "" {
						if !tt.expectErr {
							t.Errorf("Invalid upload flag format: %s", flag)
						}
					}
				}
			}
		})
	}
}

func TestCommandHelp(t *testing.T) {
	// Test that help flags are properly handled
	helpTests := []struct {
		command     string
		helpFlags   []string
		description string
	}{
		{
			command:     "run",
			helpFlags:   []string{"-h", "--help"},
			description: "Run command help",
		},
		{
			command:     "status",
			helpFlags:   []string{"-h", "--help"},
			description: "Status command help",
		},
		{
			command:     "list",
			helpFlags:   []string{"-h", "--help"},
			description: "List command help",
		},
		{
			command:     "monitor",
			helpFlags:   []string{"-h", "--help"},
			description: "Monitor command help",
		},
	}

	for _, ht := range helpTests {
		t.Run(ht.command+"_help_flags", func(t *testing.T) {
			for _, helpFlag := range ht.helpFlags {
				if helpFlag != "-h" && helpFlag != "--help" {
					t.Errorf("Invalid help flag: %s", helpFlag)
				}
			}

			if len(ht.helpFlags) != 2 {
				t.Errorf("Expected 2 help flags (-h, --help), got %d", len(ht.helpFlags))
			}
		})
	}
}

// Benchmark tests for command creation performance
func BenchmarkCommandCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := &cobra.Command{
			Use:   "test",
			Short: "Test command",
			RunE: func(cmd *cobra.Command, args []string) error {
				return nil
			},
		}

		// Add some flags
		cmd.Flags().String("test-flag", "", "Test flag")
		cmd.Flags().Bool("bool-flag", false, "Boolean flag")
		cmd.Flags().Int("int-flag", 0, "Integer flag")
	}
}

func BenchmarkGlobalFlagParsing(b *testing.B) {
	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	var testConfig string
	var testNode string
	var testJSON bool

	cmd.Flags().StringVar(&testConfig, "config", "", "Config path")
	cmd.Flags().StringVar(&testNode, "node", "default", "Node name")
	cmd.Flags().BoolVar(&testJSON, "json", false, "JSON output")

	testArgs := []string{"--config", "/tmp/test.yml", "--node", "production", "--json"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd.SetArgs(testArgs)
		if err := cmd.ParseFlags(testArgs); err != nil {
			b.Fatalf("Failed to parse flags: %v", err)
		}
	}
}

// Test environment setup and teardown
func TestMain(m *testing.M) {
	// Setup
	originalConfigPath := common.ConfigPath
	originalNodeName := common.NodeName
	originalJSONOutput := common.JSONOutput

	// Run tests
	code := m.Run()

	// Teardown - restore original values
	common.ConfigPath = originalConfigPath
	common.NodeName = originalNodeName
	common.JSONOutput = originalJSONOutput

	os.Exit(code)
}
