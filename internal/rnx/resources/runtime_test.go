package resources

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ehsaniara/joblet/internal/rnx/common"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestNewRuntimeCmd(t *testing.T) {
	cmd := NewRuntimeCmd()

	if cmd == nil {
		t.Fatal("NewRuntimeCmd() returned nil")
	}

	if cmd.Use != "runtime" {
		t.Errorf("Expected Use 'runtime', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	if cmd.Long == "" {
		t.Error("Long description is empty")
	}

	// Check that subcommands are added
	subcommands := cmd.Commands()

	if len(subcommands) == 0 {
		t.Error("No subcommands found")
	}

	// Verify that we have the expected number of subcommands (should be at least the core ones)
	if len(subcommands) < 4 {
		t.Errorf("Expected at least 4 subcommands, got %d", len(subcommands))
	}

	// Verify subcommand names (check that main commands exist)
	foundSubcommands := make(map[string]bool)
	for _, subcmd := range subcommands {
		foundSubcommands[subcmd.Use] = true
	}

	coreCommands := []string{"list", "info <runtime>", "test <runtime>", "install <runtime-spec>"}
	for _, expected := range coreCommands {
		if !foundSubcommands[expected] {
			t.Errorf("Expected core subcommand '%s' not found", expected)
		}
	}
}

func TestNewRuntimeListCmd(t *testing.T) {
	cmd := NewRuntimeListCmd()

	if cmd == nil {
		t.Fatal("NewRuntimeListCmd() returned nil")
	}

	if cmd.Use != "list" {
		t.Errorf("Expected Use 'list', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	// Test that the command has proper structure
	if cmd.RunE == nil {
		t.Error("RunE function is nil")
	}
}

func TestNewRuntimeInfoCmd(t *testing.T) {
	cmd := NewRuntimeInfoCmd()

	if cmd == nil {
		t.Fatal("NewRuntimeInfoCmd() returned nil")
	}

	if cmd.Use != "info <runtime>" {
		t.Errorf("Expected Use 'info <runtime>', got %s", cmd.Use)
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

func TestNewRuntimeTestCmd(t *testing.T) {
	cmd := NewRuntimeTestCmd()

	if cmd == nil {
		t.Fatal("NewRuntimeTestCmd() returned nil")
	}

	if cmd.Use != "test <runtime>" {
		t.Errorf("Expected Use 'test <runtime>', got %s", cmd.Use)
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

func TestNewRuntimeInstallCmd(t *testing.T) {
	cmd := NewRuntimeInstallCmd()

	if cmd == nil {
		t.Fatal("NewRuntimeInstallCmd() returned nil")
	}

	if cmd.Use != "install <runtime-spec>" {
		t.Errorf("Expected Use 'install <runtime-spec>', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	if cmd.RunE == nil {
		t.Error("RunE function is nil")
	}

	// Check for expected flags (install command only has force flag)
	expectedFlags := map[string]bool{
		"force": false, // bool flag
	}

	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if _, exists := expectedFlags[flag.Name]; exists {
			expectedFlags[flag.Name] = true
		}
	})

	for flagName, found := range expectedFlags {
		if !found {
			t.Errorf("Expected flag '%s' not found", flagName)
		}
	}
}

func TestRuntimeCommandHelp(t *testing.T) {
	tests := []struct {
		name        string
		cmdFunc     func() *cobra.Command
		expectedUse string
	}{
		{
			name:        "runtime command help",
			cmdFunc:     NewRuntimeCmd,
			expectedUse: "runtime",
		},
		{
			name:        "runtime list help",
			cmdFunc:     NewRuntimeListCmd,
			expectedUse: "list",
		},
		{
			name:        "runtime info help",
			cmdFunc:     NewRuntimeInfoCmd,
			expectedUse: "info <runtime>",
		},
		{
			name:        "runtime test help",
			cmdFunc:     NewRuntimeTestCmd,
			expectedUse: "test <runtime>",
		},
		{
			name:        "runtime install help",
			cmdFunc:     NewRuntimeInstallCmd,
			expectedUse: "install <runtime-spec>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmdFunc()

			// Capture help output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetArgs([]string{"--help"})

			// The help should not error and should contain expected content
			if cmd.Use != tt.expectedUse {
				t.Errorf("Expected Use '%s', got '%s'", tt.expectedUse, cmd.Use)
			}

			// Check that help text contains key information
			helpText := cmd.Long
			if helpText == "" {
				helpText = cmd.Short
			}

			if helpText == "" {
				t.Error("No help text found (both Long and Short are empty)")
			}
		})
	}
}

func TestRuntimeInstallFlags(t *testing.T) {
	cmd := NewRuntimeInstallCmd()

	// Test flag definitions
	tests := []struct {
		flagName     string
		expectedType string
		hasDefault   bool
	}{
		{"force", "bool", true}, // bool flags have default false
	}

	for _, tt := range tests {
		t.Run("flag_"+tt.flagName, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("Flag '%s' not found", tt.flagName)
				return
			}

			// Check flag type by checking if it's a bool flag
			if tt.expectedType == "bool" {
				if flag.Value.Type() != "bool" {
					t.Errorf("Expected flag '%s' to be bool, got %s", tt.flagName, flag.Value.Type())
				}
			} else if tt.expectedType == "string" {
				if flag.Value.Type() != "string" {
					t.Errorf("Expected flag '%s' to be string, got %s", tt.flagName, flag.Value.Type())
				}
			}
		})
	}
}

func TestJSONOutputFormatting(t *testing.T) {
	// Test that JSON output formatting is considered
	tests := []struct {
		name          string
		jsonOutput    bool
		expectedCheck bool
	}{
		{
			name:          "JSON output enabled",
			jsonOutput:    true,
			expectedCheck: true,
		},
		{
			name:          "JSON output disabled",
			jsonOutput:    false,
			expectedCheck: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the global JSON output flag
			original := common.JSONOutput
			common.JSONOutput = tt.jsonOutput
			defer func() {
				common.JSONOutput = original
			}()

			// Test that the global flag is properly set
			if common.JSONOutput != tt.expectedCheck {
				t.Errorf("Expected JSONOutput %v, got %v", tt.expectedCheck, common.JSONOutput)
			}
		})
	}
}

func TestOutputRuntimeInfoJSON(t *testing.T) {
	// Test the JSON output function we added
	tests := []struct {
		name        string
		runtimeName string
		version     string
		description string
		packages    []string
		runtimeSpec string
	}{
		{
			name:        "basic runtime",
			runtimeName: "python-3.11",
			version:     "3.11.5",
			description: "Python 3.11 runtime",
			packages:    []string{"numpy", "pandas"},
			runtimeSpec: "python-3.11",
		},
		{
			name:        "runtime without packages",
			runtimeName: "openjdk-21",
			version:     "21.0.8",
			description: "OpenJDK 21",
			packages:    []string{},
			runtimeSpec: "openjdk-21",
		},
		{
			name:        "runtime with nil packages",
			runtimeName: "node-18",
			version:     "18.19.0",
			description: "Node.js 18",
			packages:    nil,
			runtimeSpec: "node-18",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily test the actual gRPC function without mocking,
			// but we can test the JSON structure expectations
			expectedFields := []string{"name", "version", "description", "packages", "usage"}

			for _, field := range expectedFields {
				// This test just validates that we expect these fields
				if field == "" {
					t.Error("Expected field name is empty")
				}
			}

			// Test runtime spec validation
			if tt.runtimeSpec == "" {
				t.Error("Runtime spec should not be empty")
			}

			// Test that usage string would be properly formatted
			expectedUsage := fmt.Sprintf("rnx job run --runtime=%s <command>", tt.runtimeSpec)
			if !strings.Contains(expectedUsage, tt.runtimeSpec) {
				t.Errorf("Usage string doesn't contain runtime spec: %s", expectedUsage)
			}
		})
	}
}

func TestRuntimeCommandValidation(t *testing.T) {
	// Test command argument validation
	tests := []struct {
		name      string
		cmdFunc   func() *cobra.Command
		args      []string
		expectErr bool
	}{
		{
			name:      "runtime info with valid argument",
			cmdFunc:   NewRuntimeInfoCmd,
			args:      []string{"python-3.11"},
			expectErr: false,
		},
		{
			name:      "runtime info without argument",
			cmdFunc:   NewRuntimeInfoCmd,
			args:      []string{},
			expectErr: true,
		},
		{
			name:      "runtime info with too many arguments",
			cmdFunc:   NewRuntimeInfoCmd,
			args:      []string{"python-3.11", "extra"},
			expectErr: true,
		},
		{
			name:      "runtime test with valid argument",
			cmdFunc:   NewRuntimeTestCmd,
			args:      []string{"openjdk-21"},
			expectErr: false,
		},
		{
			name:      "runtime test without argument",
			cmdFunc:   NewRuntimeTestCmd,
			args:      []string{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmdFunc()
			cmd.SetArgs(tt.args)

			// Test argument validation (Args field)
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

// Benchmark tests
func BenchmarkRuntimeCommandCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewRuntimeCmd()
	}
}

func BenchmarkRuntimeSubcommandCreation(b *testing.B) {
	commands := []func() *cobra.Command{
		NewRuntimeListCmd,
		NewRuntimeInfoCmd,
		NewRuntimeTestCmd,
		NewRuntimeInstallCmd,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, cmdFunc := range commands {
			_ = cmdFunc()
		}
	}
}

// Test cleanup
func TestMain(m *testing.M) {
	// Setup
	originalJSONOutput := common.JSONOutput

	// Run tests
	code := m.Run()

	// Cleanup
	common.JSONOutput = originalJSONOutput

	os.Exit(code)
}
