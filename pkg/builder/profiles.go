//go:build linux

package builder

import (
	"fmt"
	"strings"
)

// GetLanguageProfile returns the language profile for the given language and version
func GetLanguageProfile(language, version string) (*LanguageProfile, error) {
	switch language {
	case "python":
		return getPythonProfile(version), nil
	case "java":
		return getJavaProfile(version), nil
	case "node":
		return getNodeProfile(version), nil
	case "go":
		return getGoProfile(version), nil
	case "rust":
		return getRustProfile(version), nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
}

func getPythonProfile(version string) *LanguageProfile {
	// Normalize version (e.g., "3.11" -> "3.11")
	v := strings.TrimPrefix(version, "python")

	return &LanguageProfile{
		Language: "python",
		Version:  version,
		AptPackages: []string{
			fmt.Sprintf("python%s", v),
			fmt.Sprintf("python%s-dev", v),
			fmt.Sprintf("python%s-venv", v),
			"python3-pip",
			"libssl-dev",
			"zlib1g-dev",
			"libffi-dev",
		},
		YumPackages: []string{
			fmt.Sprintf("python%s", strings.Replace(v, ".", "", 1)),
			fmt.Sprintf("python%s-devel", strings.Replace(v, ".", "", 1)),
			"python3-pip",
			"openssl-devel",
			"zlib-devel",
			"libffi-devel",
		},
		Binaries: []string{
			fmt.Sprintf("python%s", v),
			"python3",
			"pip3",
			"pip",
		},
		LibraryPatterns: []string{
			fmt.Sprintf("libpython%s*", v),
			"libssl*",
			"libcrypto*",
			"libz*",
			"libffi*",
		},
		Environment: map[string]string{
			"PYTHONUNBUFFERED": "1",
			"PYTHONDONTWRITEBYTECODE": "1",
		},
	}
}

func getJavaProfile(version string) *LanguageProfile {
	return &LanguageProfile{
		Language: "java",
		Version:  version,
		AptPackages: []string{
			fmt.Sprintf("openjdk-%s-jdk", version),
			"ca-certificates",
		},
		YumPackages: []string{
			fmt.Sprintf("java-%s-openjdk-devel", version),
			"ca-certificates",
		},
		Binaries: []string{
			"java",
			"javac",
			"jar",
		},
		LibraryPatterns: []string{
			"libjava*",
			"libjvm*",
			"libz*",
			"libpthread*",
			"libdl*",
			"librt*",
			"libm*",
			"libc*",
			"libgcc_s*",
			"libstdc++*",
		},
		Environment: map[string]string{
			"JAVA_HOME": fmt.Sprintf("/usr/lib/jvm/java-%s-openjdk-amd64", version),
		},
	}
}

func getNodeProfile(version string) *LanguageProfile {
	return &LanguageProfile{
		Language: "node",
		Version:  version,
		AptPackages: []string{
			"nodejs",
			"npm",
		},
		YumPackages: []string{
			"nodejs",
			"npm",
		},
		Binaries: []string{
			"node",
			"npm",
			"npx",
		},
		LibraryPatterns: []string{
			"libnode*",
		},
		Environment: map[string]string{
			"NODE_ENV": "production",
		},
	}
}

func getGoProfile(version string) *LanguageProfile {
	return &LanguageProfile{
		Language: "go",
		Version:  version,
		AptPackages: []string{
			"golang",
			"git",
		},
		YumPackages: []string{
			"golang",
			"git",
		},
		Binaries: []string{
			"go",
			"gofmt",
		},
		LibraryPatterns: []string{},
		Environment: map[string]string{
			"GOPATH": "/go",
			"GOBIN":  "/go/bin",
		},
	}
}

func getRustProfile(version string) *LanguageProfile {
	return &LanguageProfile{
		Language: "rust",
		Version:  version,
		AptPackages: []string{
			"rustc",
			"cargo",
		},
		YumPackages: []string{
			"rust",
			"cargo",
		},
		Binaries: []string{
			"rustc",
			"cargo",
		},
		LibraryPatterns: []string{
			"libstd-*",
		},
		Environment: map[string]string{
			"CARGO_HOME": "/cargo",
		},
	}
}
