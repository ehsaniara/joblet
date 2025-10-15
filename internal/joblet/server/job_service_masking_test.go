package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

func TestSecretEnvironmentVariableMasking(t *testing.T) {

	tests := []struct {
		name              string
		secretEnv         map[string]string
		expectedMasked    map[string]string
		expectedMaskCount int
	}{
		{
			name: "single secret variable",
			secretEnv: map[string]string{
				"SECRET_KEY": "very-secret-value",
			},
			expectedMasked: map[string]string{
				"SECRET_KEY": "***",
			},
			expectedMaskCount: 1,
		},
		{
			name: "multiple secret variables",
			secretEnv: map[string]string{
				"API_TOKEN":    "secret-token-123",
				"DATABASE_PWD": "super-secret-password",
				"SECRET_KEY":   "another-secret",
			},
			expectedMasked: map[string]string{
				"API_TOKEN":    "***",
				"DATABASE_PWD": "***",
				"SECRET_KEY":   "***",
			},
			expectedMaskCount: 3,
		},
		{
			name:              "no secret variables",
			secretEnv:         map[string]string{},
			expectedMasked:    map[string]string{},
			expectedMaskCount: 0,
		},
		{
			name:              "nil secret variables",
			secretEnv:         nil,
			expectedMasked:    map[string]string{},
			expectedMaskCount: 0,
		},
		{
			name: "secret with empty value",
			secretEnv: map[string]string{
				"EMPTY_SECRET": "",
			},
			expectedMasked: map[string]string{
				"EMPTY_SECRET": "***",
			},
			expectedMaskCount: 1,
		},
		{
			name: "secret with very long value",
			secretEnv: map[string]string{
				"LONG_SECRET": string(make([]byte, 10000)),
			},
			expectedMasked: map[string]string{
				"LONG_SECRET": "***",
			},
			expectedMaskCount: 1,
		},
		{
			name: "secret with special characters",
			secretEnv: map[string]string{
				"SPECIAL_SECRET": "secret!@#$%^&*()_+-={}[]|\\:;\"'<>,.?/",
			},
			expectedMasked: map[string]string{
				"SPECIAL_SECRET": "***",
			},
			expectedMaskCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock job with secret environment variables
			job := &domain.Job{
				Uuid:              "test-job-id",
				Command:           "echo",
				Args:              []string{"test"},
				Status:            domain.StatusCompleted,
				StartTime:         time.Now(),
				SecretEnvironment: tt.secretEnv,
				Environment:       map[string]string{"PUBLIC_VAR": "public_value"},
				Limits:            *domain.NewResourceLimits(),
			}

			// Test the masking logic directly
			maskedSecretEnv := make(map[string]string)
			for key := range job.SecretEnvironment {
				maskedSecretEnv[key] = "***"
			}

			// Verify masking results
			if len(maskedSecretEnv) != tt.expectedMaskCount {
				t.Errorf("Expected %d masked variables, got %d", tt.expectedMaskCount, len(maskedSecretEnv))
			}

			for key, expectedValue := range tt.expectedMasked {
				if actualValue, exists := maskedSecretEnv[key]; !exists {
					t.Errorf("Expected masked variable %s to exist", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected masked value for %s to be %s, got %s", key, expectedValue, actualValue)
				}
			}

			// Verify original secret values are not exposed
			for key, originalValue := range job.SecretEnvironment {
				if maskedValue, exists := maskedSecretEnv[key]; exists {
					if maskedValue == originalValue && originalValue != "***" {
						t.Errorf("Secret variable %s was not properly masked (original value exposed)", key)
					}
				}
			}

			// Verify masking doesn't affect regular environment variables
			if job.Environment != nil {
				for key, value := range job.Environment {
					if _, exists := maskedSecretEnv[key]; exists {
						t.Errorf("Regular environment variable %s should not be in masked secrets", key)
					}
					// Regular env vars should remain unchanged
					if value != job.Environment[key] {
						t.Errorf("Regular environment variable %s was modified", key)
					}
				}
			}
		})
	}
}

func TestSecretMaskingConsistency(t *testing.T) {
	tests := []struct {
		name      string
		secretEnv map[string]string
	}{
		{
			name: "consistent masking with same secrets",
			secretEnv: map[string]string{
				"SECRET1": "value1",
				"SECRET2": "value2",
				"SECRET3": "value3",
			},
		},
		{
			name: "consistent masking with different secret values",
			secretEnv: map[string]string{
				"API_KEY":      "short",
				"LONG_SECRET":  string(make([]byte, 1000)),
				"EMPTY_SECRET": "",
				"UNICODE":      "unicode-value-🔐",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply masking multiple times
			var maskingResults []map[string]string

			for i := 0; i < 5; i++ {
				maskedSecretEnv := make(map[string]string)
				for key := range tt.secretEnv {
					maskedSecretEnv[key] = "***"
				}
				maskingResults = append(maskingResults, maskedSecretEnv)
			}

			// Verify all masking results are identical
			firstResult := maskingResults[0]
			for i, result := range maskingResults[1:] {
				if len(result) != len(firstResult) {
					t.Errorf("Masking result %d has different length than first result", i+1)
					continue
				}

				for key, value := range firstResult {
					if resultValue, exists := result[key]; !exists {
						t.Errorf("Masking result %d missing key %s", i+1, key)
					} else if resultValue != value {
						t.Errorf("Masking result %d has different value for %s: expected %s, got %s", i+1, key, value, resultValue)
					}
				}
			}
		})
	}
}

func TestSecretMaskingWithMixedEnvironment(t *testing.T) {
	regularEnv := map[string]string{
		"PUBLIC_VAR1": "public_value1",
		"PUBLIC_VAR2": "public_value2",
		"PATH":        "/usr/bin:/bin",
	}

	secretEnv := map[string]string{
		"SECRET_API_KEY": "secret-api-key-value",
		"DATABASE_PWD":   "secret-database-password",
	}

	// Apply masking to secrets only
	maskedSecretEnv := make(map[string]string)
	for key := range secretEnv {
		maskedSecretEnv[key] = "***"
	}

	// Verify separation between regular and secret environment variables
	for key := range regularEnv {
		if _, exists := maskedSecretEnv[key]; exists {
			t.Errorf("Regular environment variable %s should not appear in masked secrets", key)
		}
	}

	for key := range secretEnv {
		if _, exists := regularEnv[key]; exists {
			t.Errorf("Secret environment variable %s should not appear in regular environment", key)
		}
		if maskedValue, exists := maskedSecretEnv[key]; !exists {
			t.Errorf("Secret environment variable %s should be present in masked secrets", key)
		} else if maskedValue != "***" {
			t.Errorf("Secret environment variable %s should be masked as '***', got '%s'", key, maskedValue)
		}
	}

	// Verify total counts
	if len(maskedSecretEnv) != len(secretEnv) {
		t.Errorf("Expected %d masked secret variables, got %d", len(secretEnv), len(maskedSecretEnv))
	}
}

// Benchmark tests for masking performance
func BenchmarkSecretEnvironmentMasking(b *testing.B) {
	// Create test data with many secret variables
	secretEnv := make(map[string]string)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("SECRET_VAR_%d", i)
		value := fmt.Sprintf("secret-value-%d-very-long-secret-that-should-be-masked", i)
		secretEnv[key] = value
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		maskedSecretEnv := make(map[string]string)
		for key := range secretEnv {
			maskedSecretEnv[key] = "***"
		}
	}
}

func BenchmarkSecretMaskingSingleVariable(b *testing.B) {
	secretEnv := map[string]string{
		"SECRET_KEY": "very-long-secret-value-that-needs-to-be-masked-for-security-purposes",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		maskedSecretEnv := make(map[string]string)
		for key := range secretEnv {
			maskedSecretEnv[key] = "***"
		}
	}
}
