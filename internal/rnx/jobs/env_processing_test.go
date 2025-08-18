package jobs

import (
	"testing"
)

func TestProcessEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name        string
		envVars     []string
		expected    map[string]string
		expectError bool
	}{
		{
			name:    "single environment variable",
			envVars: []string{"TEST_VAR=test_value"},
			expected: map[string]string{
				"TEST_VAR": "test_value",
			},
			expectError: false,
		},
		{
			name:    "multiple environment variables",
			envVars: []string{"VAR1=value1", "VAR2=value2", "VAR3=value3"},
			expected: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
				"VAR3": "value3",
			},
			expectError: false,
		},
		{
			name:    "empty value",
			envVars: []string{"EMPTY_VAR="},
			expected: map[string]string{
				"EMPTY_VAR": "",
			},
			expectError: false,
		},
		{
			name:    "value with spaces",
			envVars: []string{"SPACE_VAR=value with spaces"},
			expected: map[string]string{
				"SPACE_VAR": "value with spaces",
			},
			expectError: false,
		},
		{
			name:    "value with special characters",
			envVars: []string{"SPECIAL_VAR=value!@#$%^&*()"},
			expected: map[string]string{
				"SPECIAL_VAR": "value!@#$%^&*()",
			},
			expectError: false,
		},
		{
			name:    "value with equals sign",
			envVars: []string{"EQUALS_VAR=key=value=more"},
			expected: map[string]string{
				"EQUALS_VAR": "key=value=more",
			},
			expectError: false,
		},
		{
			name:        "missing equals sign",
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
		{
			name:        "empty string",
			envVars:     []string{""},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processEnvironmentVariables(tt.envVars)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input %v, but got none", tt.envVars)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for input %v: %v", tt.envVars, err)
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

func TestValidateEnvironmentVariableName(t *testing.T) {
	tests := []struct {
		name        string
		varName     string
		expectError bool
	}{
		// Valid names
		{
			name:        "simple name",
			varName:     "TEST_VAR",
			expectError: false,
		},
		{
			name:        "name with numbers",
			varName:     "VAR123",
			expectError: false,
		},
		{
			name:        "name starting with underscore",
			varName:     "_PRIVATE_VAR",
			expectError: false,
		},
		{
			name:        "lowercase name",
			varName:     "lowercase_var",
			expectError: false,
		},
		{
			name:        "mixed case name",
			varName:     "MixedCase_Var123",
			expectError: false,
		},
		{
			name:        "single character",
			varName:     "A",
			expectError: false,
		},
		{
			name:        "underscore only",
			varName:     "_",
			expectError: false,
		},

		// Invalid names
		{
			name:        "starts with number",
			varName:     "123_VAR",
			expectError: true,
		},
		{
			name:        "contains hyphen",
			varName:     "INVALID-VAR",
			expectError: true,
		},
		{
			name:        "contains space",
			varName:     "INVALID VAR",
			expectError: true,
		},
		{
			name:        "contains special characters",
			varName:     "INVALID@VAR",
			expectError: true,
		},
		{
			name:        "contains dot",
			varName:     "INVALID.VAR",
			expectError: true,
		},
		{
			name:        "empty name",
			varName:     "",
			expectError: true,
		},
		{
			name:        "starts with special character",
			varName:     "@INVALID",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvironmentVariableName(tt.varName)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for variable name '%s', but got none", tt.varName)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for variable name '%s': %v", tt.varName, err)
				}
			}
		})
	}
}

func TestValidateEnvironmentVariableValue(t *testing.T) {
	tests := []struct {
		name        string
		varName     string
		value       string
		expectError bool
	}{
		// Valid values
		{
			name:        "simple value",
			varName:     "TEST_VAR",
			value:       "simple_value",
			expectError: false,
		},
		{
			name:        "empty value",
			varName:     "EMPTY_VAR",
			value:       "",
			expectError: false,
		},
		{
			name:        "value with spaces",
			varName:     "SPACE_VAR",
			value:       "value with spaces",
			expectError: false,
		},
		{
			name:        "value with special characters",
			varName:     "SPECIAL_VAR",
			value:       "value!@#$%^&*()",
			expectError: false,
		},
		{
			name:        "value with newlines",
			varName:     "NEWLINE_VAR",
			value:       "line1\nline2",
			expectError: false,
		},
		{
			name:        "value with unicode",
			varName:     "UNICODE_VAR",
			value:       "value with unicode: 你好",
			expectError: false,
		},
		{
			name:        "normal long value",
			varName:     "LONG_VAR",
			value:       string(make([]byte, 10000)),
			expectError: false,
		},
		{
			name:        "maximum allowed length",
			varName:     "MAX_VAR",
			value:       string(make([]byte, 32768)),
			expectError: false,
		},

		// Invalid values (too long)
		{
			name:        "value too long",
			varName:     "TOO_LONG_VAR",
			value:       string(make([]byte, 32769)), // 1 byte over limit
			expectError: true,
		},
		{
			name:        "extremely long value",
			varName:     "EXTREME_VAR",
			value:       string(make([]byte, 100000)),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvironmentVariableValue(tt.varName, tt.value)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for value length %d, but got none", len(tt.value))
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for value '%s': %v", tt.value, err)
				}
			}
		})
	}
}

// Test the integration of environment variable processing in the CLI
func TestEnvironmentVariableIntegration(t *testing.T) {
	tests := []struct {
		name                string
		envVars             []string
		secretEnvVars       []string
		expectedEnvCount    int
		expectedSecretCount int
		expectError         bool
	}{
		{
			name:                "both regular and secret variables",
			envVars:             []string{"PUBLIC_VAR=public_value"},
			secretEnvVars:       []string{"SECRET_VAR=secret_value"},
			expectedEnvCount:    1,
			expectedSecretCount: 1,
			expectError:         false,
		},
		{
			name:                "only regular variables",
			envVars:             []string{"VAR1=value1", "VAR2=value2"},
			secretEnvVars:       []string{},
			expectedEnvCount:    2,
			expectedSecretCount: 0,
			expectError:         false,
		},
		{
			name:                "only secret variables",
			envVars:             []string{},
			secretEnvVars:       []string{"SECRET1=secret1", "SECRET2=secret2"},
			expectedEnvCount:    0,
			expectedSecretCount: 2,
			expectError:         false,
		},
		{
			name:                "no variables",
			envVars:             []string{},
			secretEnvVars:       []string{},
			expectedEnvCount:    0,
			expectedSecretCount: 0,
			expectError:         false,
		},
		{
			name:                "invalid regular variable",
			envVars:             []string{"INVALID-VAR=value"},
			secretEnvVars:       []string{},
			expectedEnvCount:    0,
			expectedSecretCount: 0,
			expectError:         true,
		},
		{
			name:                "invalid secret variable",
			envVars:             []string{},
			secretEnvVars:       []string{"INVALID-SECRET=secret"},
			expectedEnvCount:    0,
			expectedSecretCount: 0,
			expectError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Process regular environment variables
			envMap, envErr := processEnvironmentVariables(tt.envVars)

			// Process secret environment variables
			secretMap, secretErr := processEnvironmentVariables(tt.secretEnvVars)

			if tt.expectError {
				if envErr == nil && secretErr == nil {
					t.Errorf("Expected error, but got none")
				}
				return
			}

			if envErr != nil {
				t.Errorf("Unexpected error processing regular variables: %v", envErr)
				return
			}
			if secretErr != nil {
				t.Errorf("Unexpected error processing secret variables: %v", secretErr)
				return
			}

			if len(envMap) != tt.expectedEnvCount {
				t.Errorf("Expected %d regular variables, got %d", tt.expectedEnvCount, len(envMap))
			}
			if len(secretMap) != tt.expectedSecretCount {
				t.Errorf("Expected %d secret variables, got %d", tt.expectedSecretCount, len(secretMap))
			}
		})
	}
}

// Benchmark tests for performance
func BenchmarkProcessEnvironmentVariables(b *testing.B) {
	envVars := []string{
		"VAR1=value1",
		"VAR2=value2_with_longer_value",
		"VAR3=value3",
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/home/user",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processEnvironmentVariables(envVars)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkValidateEnvironmentVariableName(b *testing.B) {
	varName := "VALID_ENVIRONMENT_VARIABLE_NAME_123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := validateEnvironmentVariableName(varName)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}
