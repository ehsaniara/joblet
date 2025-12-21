//go:build linux

package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// RuntimeConfig represents the generated runtime.yml for the joblet server
type RuntimeConfig struct {
	Name         string            `yaml:"name"`
	Language     string            `yaml:"language"`
	Version      string            `yaml:"version"`
	Description  string            `yaml:"description"`
	Mounts       []MountSpec       `yaml:"mounts"`
	Environment  map[string]string `yaml:"environment,omitempty"`
	Packages     []string          `yaml:"packages,omitempty"`
	Requirements RuntimeRequirements `yaml:"requirements,omitempty"`
	BuildInfo    BuildInfoSpec     `yaml:"build_info"`
}

// MountSpec defines a mount point
type MountSpec struct {
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"readonly"`
}

// RuntimeRequirements defines runtime requirements
type RuntimeRequirements struct {
	Architectures []string `yaml:"architectures,omitempty"`
	GPU           bool     `yaml:"gpu,omitempty"`
	CUDAVersion   string   `yaml:"cuda_version,omitempty"`
}

// BuildInfoSpec contains build metadata
type BuildInfoSpec struct {
	BuiltAt   string `yaml:"built_at"`
	BuiltWith string `yaml:"built_with"`
	Platform  string `yaml:"platform"`
}

// GenerateRuntimeConfig generates the runtime.yml file for the joblet server
func GenerateRuntimeConfig(buildCtx *BuildContext, logger BuildLogger) error {
	// Generate mounts for each directory that exists in the isolated directory
	mounts := generateMounts(buildCtx.IsolatedDir)

	config := RuntimeConfig{
		Name:        buildCtx.Spec.Name,
		Language:    buildCtx.Spec.Base.Language,
		Version:     buildCtx.Spec.Version,
		Description: buildCtx.Spec.Description,
		Mounts:      mounts,
		Environment: generateEnvironment(buildCtx),
		BuildInfo: BuildInfoSpec{
			BuiltAt:   time.Now().UTC().Format(time.RFC3339),
			BuiltWith: "joblet-builder",
			Platform:  buildCtx.Platform.GetPlatformString(),
		},
	}

	// Add packages to the config
	if len(buildCtx.Spec.Pip) > 0 {
		config.Packages = append(config.Packages, buildCtx.Spec.Pip...)
	}
	if len(buildCtx.Spec.Npm) > 0 {
		config.Packages = append(config.Packages, buildCtx.Spec.Npm...)
	}

	// Add requirements
	if buildCtx.Spec.Requirements != nil {
		config.Requirements = RuntimeRequirements{
			GPU:         buildCtx.Spec.Requirements.GPU,
			CUDAVersion: buildCtx.Spec.Requirements.CUDAVersion,
		}
	}

	// Set architectures
	config.Requirements.Architectures = []string{buildCtx.Platform.Arch}

	// Marshal to YAML
	data, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal runtime config: %w", err)
	}

	// Write to file
	configPath := filepath.Join(buildCtx.RuntimeDir, "runtime.yml")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write runtime config: %w", err)
	}

	logger.Info("Generated runtime config: %s", configPath)
	return nil
}

// generateMounts creates mount specifications for each directory in the isolated directory
func generateMounts(isolatedDir string) []MountSpec {
	// Standard directory mappings
	dirMappings := []struct {
		subDir   string
		target   string
		readOnly bool
	}{
		{"usr/local/bin", "/usr/local/bin", true},
		{"usr/local/lib", "/usr/local/lib", true},
		{"usr/bin", "/usr/bin", true},
		{"bin", "/bin", true},
		{"sbin", "/sbin", true},
		{"usr/sbin", "/usr/sbin", true},
		{"lib", "/lib", true},
		{"lib64", "/lib64", true},
		{"usr/lib", "/usr/lib", true},
		{"lib/x86_64-linux-gnu", "/lib/x86_64-linux-gnu", true},
		{"usr/lib/x86_64-linux-gnu", "/usr/lib/x86_64-linux-gnu", true},
		{"usr/lib/jvm", "/usr/lib/jvm", true},
		{"etc/ssl", "/etc/ssl", true},
		{"etc/pki", "/etc/pki", true},
		{"etc/ca-certificates", "/etc/ca-certificates", true},
		{"usr/share/ca-certificates", "/usr/share/ca-certificates", true},
		{"tmp", "/tmp", false},
		{"var/tmp", "/var/tmp", false},
	}

	var mounts []MountSpec

	for _, dm := range dirMappings {
		fullPath := filepath.Join(isolatedDir, dm.subDir)
		if _, err := os.Stat(fullPath); err == nil {
			// Use relative path from runtime directory
			mounts = append(mounts, MountSpec{
				Source:   "isolated/" + dm.subDir,
				Target:   dm.target,
				ReadOnly: dm.readOnly,
			})
		}
	}

	return mounts
}

// generateEnvironment creates environment variables for the runtime
func generateEnvironment(buildCtx *BuildContext) map[string]string {
	env := make(map[string]string)

	// Copy user-defined environment
	for k, v := range buildCtx.Spec.Environment {
		env[k] = v
	}

	// Add PATH if not already set
	if _, ok := env["PATH"]; !ok {
		env["PATH"] = "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
	}

	// Add language-specific environment
	switch buildCtx.Spec.Base.Language {
	case "python":
		if _, ok := env["PYTHONUNBUFFERED"]; !ok {
			env["PYTHONUNBUFFERED"] = "1"
		}
		// Set PYTHONPATH for pip packages
		if _, ok := env["PYTHONPATH"]; !ok {
			env["PYTHONPATH"] = "/usr/local/lib/python3/site-packages"
		}
	case "java":
		if _, ok := env["JAVA_HOME"]; !ok {
			// Try to detect JAVA_HOME
			javaHome := detectJavaHome(buildCtx.IsolatedDir)
			if javaHome != "" {
				env["JAVA_HOME"] = javaHome
			}
		}
	case "node":
		if _, ok := env["NODE_ENV"]; !ok {
			env["NODE_ENV"] = "production"
		}
	}

	return env
}

// detectJavaHome tries to find JAVA_HOME in the isolated directory
func detectJavaHome(isolatedDir string) string {
	// Look for java binary and derive JAVA_HOME
	javaPaths := []string{
		"usr/lib/jvm/java-21-openjdk-amd64",
		"usr/lib/jvm/java-17-openjdk-amd64",
		"usr/lib/jvm/java-11-openjdk-amd64",
		"usr/lib/jvm/default-java",
	}

	for _, javaPath := range javaPaths {
		fullPath := filepath.Join(isolatedDir, javaPath)
		if _, err := os.Stat(fullPath); err == nil {
			return "/" + javaPath
		}
	}

	return ""
}
