package server

import (
	"joblet/internal/joblet/workflow/types"
	"joblet/pkg/logger"
	"testing"
)

func TestMergeEnvironmentVariables(t *testing.T) {
	// Create a workflow service for testing
	server := &WorkflowServiceServer{
		logger: logger.WithField("component", "test"),
	}

	tests := []struct {
		name              string
		workflowYAML      *types.WorkflowYAML
		jobSpec           types.JobSpec
		expectedEnv       map[string]string
		expectedSecretEnv map[string]string
	}{
		{
			name: "basic inheritance",
			workflowYAML: &types.WorkflowYAML{
				Environment: map[string]string{
					"GLOBAL_VAR": "global_value",
					"SHARED_VAR": "workflow_value",
				},
				SecretEnvironment: map[string]string{
					"GLOBAL_SECRET": "global_secret_value",
				},
			},
			jobSpec: types.JobSpec{
				Environment: map[string]string{
					"JOB_VAR": "job_value",
				},
				SecretEnvironment: map[string]string{
					"JOB_SECRET": "job_secret_value",
				},
			},
			expectedEnv: map[string]string{
				"GLOBAL_VAR": "global_value",
				"SHARED_VAR": "workflow_value",
				"JOB_VAR":    "job_value",
			},
			expectedSecretEnv: map[string]string{
				"GLOBAL_SECRET": "global_secret_value",
				"JOB_SECRET":    "job_secret_value",
			},
		},
		{
			name: "job overrides global",
			workflowYAML: &types.WorkflowYAML{
				Environment: map[string]string{
					"OVERRIDE_VAR": "global_value",
					"KEEP_VAR":     "global_keep",
				},
				SecretEnvironment: map[string]string{
					"SECRET_OVERRIDE": "global_secret",
				},
			},
			jobSpec: types.JobSpec{
				Environment: map[string]string{
					"OVERRIDE_VAR": "job_override_value",
				},
				SecretEnvironment: map[string]string{
					"SECRET_OVERRIDE": "job_secret_override",
				},
			},
			expectedEnv: map[string]string{
				"OVERRIDE_VAR": "job_override_value", // Job overrides global
				"KEEP_VAR":     "global_keep",        // Global preserved
			},
			expectedSecretEnv: map[string]string{
				"SECRET_OVERRIDE": "job_secret_override", // Job overrides global secret
			},
		},
		{
			name: "empty global environment",
			workflowYAML: &types.WorkflowYAML{
				Environment:       nil,
				SecretEnvironment: nil,
			},
			jobSpec: types.JobSpec{
				Environment: map[string]string{
					"JOB_ONLY": "job_value",
				},
				SecretEnvironment: map[string]string{
					"SECRET_ONLY": "secret_value",
				},
			},
			expectedEnv: map[string]string{
				"JOB_ONLY": "job_value",
			},
			expectedSecretEnv: map[string]string{
				"SECRET_ONLY": "secret_value",
			},
		},
		{
			name: "empty job environment",
			workflowYAML: &types.WorkflowYAML{
				Environment: map[string]string{
					"GLOBAL_ONLY": "global_value",
				},
				SecretEnvironment: map[string]string{
					"GLOBAL_SECRET_ONLY": "global_secret",
				},
			},
			jobSpec: types.JobSpec{
				Environment:       nil,
				SecretEnvironment: nil,
			},
			expectedEnv: map[string]string{
				"GLOBAL_ONLY": "global_value",
			},
			expectedSecretEnv: map[string]string{
				"GLOBAL_SECRET_ONLY": "global_secret",
			},
		},
		{
			name: "both empty",
			workflowYAML: &types.WorkflowYAML{
				Environment:       nil,
				SecretEnvironment: nil,
			},
			jobSpec: types.JobSpec{
				Environment:       nil,
				SecretEnvironment: nil,
			},
			expectedEnv:       map[string]string{},
			expectedSecretEnv: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergedEnv, mergedSecretEnv := server.mergeEnvironmentVariables(tt.workflowYAML, tt.jobSpec)

			// Check regular environment variables
			if len(mergedEnv) != len(tt.expectedEnv) {
				t.Errorf("Expected %d environment variables, got %d", len(tt.expectedEnv), len(mergedEnv))
			}
			for key, expectedValue := range tt.expectedEnv {
				if actualValue, exists := mergedEnv[key]; !exists {
					t.Errorf("Expected environment variable %s to exist", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected %s=%s, got %s=%s", key, expectedValue, key, actualValue)
				}
			}

			// Check secret environment variables
			if len(mergedSecretEnv) != len(tt.expectedSecretEnv) {
				t.Errorf("Expected %d secret environment variables, got %d", len(tt.expectedSecretEnv), len(mergedSecretEnv))
			}
			for key, expectedValue := range tt.expectedSecretEnv {
				if actualValue, exists := mergedSecretEnv[key]; !exists {
					t.Errorf("Expected secret environment variable %s to exist", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected secret %s=%s, got %s=%s", key, expectedValue, key, actualValue)
				}
			}
		})
	}
}

func TestProcessEnvironmentTemplating(t *testing.T) {
	server := &WorkflowServiceServer{
		logger: logger.WithField("component", "test"),
	}

	tests := []struct {
		name          string
		value         string
		envVars       map[string]string
		secretEnvVars map[string]string
		expected      string
	}{
		{
			name:  "no templating",
			value: "simple_value",
			envVars: map[string]string{
				"UNUSED": "unused_value",
			},
			secretEnvVars: map[string]string{},
			expected:      "simple_value",
		},
		{
			name:  "single variable reference",
			value: "${BASE_PATH}",
			envVars: map[string]string{
				"BASE_PATH": "/opt/data",
			},
			secretEnvVars: map[string]string{},
			expected:      "/opt/data",
		},
		{
			name:  "multiple variable references",
			value: "${BASE_PATH}/v${VERSION}/output",
			envVars: map[string]string{
				"BASE_PATH": "/opt/data",
				"VERSION":   "1.2.0",
			},
			secretEnvVars: map[string]string{},
			expected:      "/opt/data/v1.2.0/output",
		},
		{
			name:  "mixed text and variables",
			value: "prefix-${VAR1}-middle-${VAR2}-suffix",
			envVars: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
			secretEnvVars: map[string]string{},
			expected:      "prefix-value1-middle-value2-suffix",
		},
		{
			name:    "secret variable reference",
			value:   "secret-${SECRET_KEY}",
			envVars: map[string]string{},
			secretEnvVars: map[string]string{
				"SECRET_KEY": "secret_value",
			},
			expected: "secret-secret_value",
		},
		{
			name:  "mixed regular and secret variables",
			value: "${PUBLIC_VAR}-${SECRET_VAR}",
			envVars: map[string]string{
				"PUBLIC_VAR": "public",
			},
			secretEnvVars: map[string]string{
				"SECRET_VAR": "secret",
			},
			expected: "public-secret",
		},
		{
			name:  "undefined variable reference",
			value: "${UNDEFINED_VAR}",
			envVars: map[string]string{
				"OTHER_VAR": "other_value",
			},
			secretEnvVars: map[string]string{},
			expected:      "${UNDEFINED_VAR}", // Should remain unchanged
		},
		{
			name:  "partially undefined variables",
			value: "${DEFINED_VAR}-${UNDEFINED_VAR}",
			envVars: map[string]string{
				"DEFINED_VAR": "defined",
			},
			secretEnvVars: map[string]string{},
			expected:      "defined-${UNDEFINED_VAR}",
		},
		{
			name:  "empty variable value",
			value: "${EMPTY_VAR}",
			envVars: map[string]string{
				"EMPTY_VAR": "",
			},
			secretEnvVars: map[string]string{},
			expected:      "",
		},
		{
			name:  "complex path templating",
			value: "${BASE_DIR}/${PROJECT}/${ENV}/logs/${SERVICE}.log",
			envVars: map[string]string{
				"BASE_DIR": "/var/log",
				"PROJECT":  "myapp",
				"ENV":      "production",
				"SERVICE":  "api",
			},
			secretEnvVars: map[string]string{},
			expected:      "/var/log/myapp/production/logs/api.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.processEnvironmentTemplating(tt.value, tt.envVars, tt.secretEnvVars)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestMergeEnvironmentVariablesWithTemplating(t *testing.T) {
	server := &WorkflowServiceServer{
		logger: logger.WithField("component", "test"),
	}

	workflowYAML := &types.WorkflowYAML{
		Environment: map[string]string{
			"BASE_PATH": "/opt/data",
			"VERSION":   "1.0.0",
		},
		SecretEnvironment: map[string]string{
			"API_KEY": "secret-api-key",
		},
	}

	jobSpec := types.JobSpec{
		Environment: map[string]string{
			"WORK_DIR":      "${BASE_PATH}/work",
			"VERSION_FILE":  "${BASE_PATH}/v${VERSION}/version.txt",
			"CONFIG_PREFIX": "app-${VERSION}",
		},
		SecretEnvironment: map[string]string{
			"SECRET_FILE": "${BASE_PATH}/secrets/${API_KEY}.key",
		},
	}

	mergedEnv, mergedSecretEnv := server.mergeEnvironmentVariables(workflowYAML, jobSpec)

	expectedEnv := map[string]string{
		"BASE_PATH":     "/opt/data",
		"VERSION":       "1.0.0",
		"WORK_DIR":      "/opt/data/work",
		"VERSION_FILE":  "/opt/data/v1.0.0/version.txt",
		"CONFIG_PREFIX": "app-1.0.0",
	}

	expectedSecretEnv := map[string]string{
		"API_KEY":     "secret-api-key",
		"SECRET_FILE": "/opt/data/secrets/secret-api-key.key",
	}

	// Verify regular environment variables
	if len(mergedEnv) != len(expectedEnv) {
		t.Errorf("Expected %d environment variables, got %d", len(expectedEnv), len(mergedEnv))
	}
	for key, expectedValue := range expectedEnv {
		if actualValue, exists := mergedEnv[key]; !exists {
			t.Errorf("Expected environment variable %s to exist", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected %s=%s, got %s=%s", key, expectedValue, key, actualValue)
		}
	}

	// Verify secret environment variables
	if len(mergedSecretEnv) != len(expectedSecretEnv) {
		t.Errorf("Expected %d secret environment variables, got %d", len(expectedSecretEnv), len(mergedSecretEnv))
	}
	for key, expectedValue := range expectedSecretEnv {
		if actualValue, exists := mergedSecretEnv[key]; !exists {
			t.Errorf("Expected secret environment variable %s to exist", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected secret %s=%s, got %s=%s", key, expectedValue, key, actualValue)
		}
	}
}

// Benchmark tests
func BenchmarkMergeEnvironmentVariables(b *testing.B) {
	server := &WorkflowServiceServer{
		logger: logger.WithField("component", "benchmark"),
	}

	workflowYAML := &types.WorkflowYAML{
		Environment: map[string]string{
			"VAR1": "value1",
			"VAR2": "value2",
			"VAR3": "value3",
			"VAR4": "value4",
			"VAR5": "value5",
		},
		SecretEnvironment: map[string]string{
			"SECRET1": "secret1",
			"SECRET2": "secret2",
			"SECRET3": "secret3",
		},
	}

	jobSpec := types.JobSpec{
		Environment: map[string]string{
			"JOB_VAR1": "job_value1",
			"JOB_VAR2": "job_value2",
			"VAR1":     "override_value1", // Override global
		},
		SecretEnvironment: map[string]string{
			"JOB_SECRET": "job_secret",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = server.mergeEnvironmentVariables(workflowYAML, jobSpec)
	}
}

func BenchmarkProcessEnvironmentTemplating(b *testing.B) {
	server := &WorkflowServiceServer{
		logger: logger.WithField("component", "benchmark"),
	}

	value := "${BASE_PATH}/projects/${PROJECT}/v${VERSION}/output/${FILE}.log"
	envVars := map[string]string{
		"BASE_PATH": "/var/log",
		"PROJECT":   "myapp",
		"VERSION":   "1.0.0",
		"FILE":      "application",
	}
	secretEnvVars := map[string]string{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.processEnvironmentTemplating(value, envVars, secretEnvVars)
	}
}
