//go:build linux

package builder

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Essential binaries that are always needed
var essentialBinaries = []string{
	"bash",
	"sh",
	"env",
	"cat",
	"ls",
	"mkdir",
	"rm",
	"cp",
	"mv",
	"chmod",
	"chown",
	"ln",
}

// Essential libraries that are always needed
var essentialLibraryPatterns = []string{
	"libc.so*",
	"libc-*.so",
	"libm.so*",
	"libm-*.so",
	"libpthread.so*",
	"libpthread-*.so",
	"libdl.so*",
	"libdl-*.so",
	"librt.so*",
	"librt-*.so",
	"ld-linux*.so*",
}

// CopyBinaries copies essential and language-specific binaries to the isolated directory
func CopyBinaries(platform *PlatformInfo, profile *LanguageProfile, isolatedDir string, logger BuildLogger) error {
	binDir := filepath.Join(isolatedDir, "usr", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Collect all binaries to copy
	binaries := make([]string, 0)
	binaries = append(binaries, essentialBinaries...)
	if profile != nil {
		binaries = append(binaries, profile.Binaries...)
	}

	// Track which versioned binaries we copied for symlink fixup
	copiedBinaries := make(map[string]bool)

	for _, binary := range binaries {
		// Find the binary path
		binaryPath, err := exec.LookPath(binary)
		if err != nil {
			logger.Warn("Binary not found, skipping: %s", binary)
			continue
		}

		// Resolve symlinks
		realPath, err := filepath.EvalSymlinks(binaryPath)
		if err != nil {
			logger.Warn("Failed to resolve symlink for %s: %v", binary, err)
			realPath = binaryPath
		}

		// Copy the binary
		destPath := filepath.Join(binDir, filepath.Base(binaryPath))
		if err := copyFile(realPath, destPath); err != nil {
			return fmt.Errorf("failed to copy binary %s: %w", binary, err)
		}

		// Track which binaries we copied
		copiedBinaries[binary] = true

		// Create symlink if original was a symlink (but we'll fix these up later for runtimes)
		if binaryPath != realPath {
			linkPath := filepath.Join(binDir, filepath.Base(binaryPath))
			realName := filepath.Base(realPath)
			os.Remove(linkPath) // Remove if exists
			if err := os.Symlink(realName, linkPath); err != nil {
				logger.Warn("Failed to create symlink for %s: %v", binary, err)
			}
		}

		logger.Debug("Copied binary: %s", binary)
	}

	// Fix up symlinks for language runtimes (e.g., python3 -> python3.11)
	if profile != nil {
		fixupSymlinks(binDir, profile, copiedBinaries, logger)
	}

	return nil
}

// fixupSymlinks ensures generic symlinks (python3, node, etc.) point to the correct versioned binary
func fixupSymlinks(binDir string, profile *LanguageProfile, copiedBinaries map[string]bool, logger BuildLogger) {
	// Define symlink mappings based on language
	symlinks := make(map[string]string)

	switch profile.Language {
	case "python":
		// Find the versioned python we copied (e.g., python3.11)
		for binary := range copiedBinaries {
			if strings.HasPrefix(binary, "python3.") && len(binary) > 8 {
				// Found a versioned python (python3.X or python3.XX)
				symlinks["python3"] = binary
				symlinks["python"] = binary
				break
			}
		}
	case "node":
		for binary := range copiedBinaries {
			if strings.HasPrefix(binary, "node") && binary != "node" {
				symlinks["node"] = binary
				break
			}
		}
	case "java":
		for binary := range copiedBinaries {
			if strings.HasPrefix(binary, "java") && binary != "java" {
				symlinks["java"] = binary
				break
			}
		}
	}

	// Create the symlinks
	for linkName, target := range symlinks {
		linkPath := filepath.Join(binDir, linkName)
		// Remove existing file/symlink
		os.Remove(linkPath)
		if err := os.Symlink(target, linkPath); err != nil {
			logger.Warn("Failed to create symlink %s -> %s: %v", linkName, target, err)
		} else {
			logger.Debug("Created symlink: %s -> %s", linkName, target)
		}
	}
}

// CopyJavaRuntime copies the entire JVM installation for Java runtimes
func CopyJavaRuntime(profile *LanguageProfile, isolatedDir string, logger BuildLogger) error {
	if profile == nil || profile.Language != "java" {
		return nil
	}

	// Find the JVM installation directory
	jvmPaths := []string{
		fmt.Sprintf("/usr/lib/jvm/java-%s-openjdk-amd64", profile.Version),
		fmt.Sprintf("/usr/lib/jvm/java-%s-openjdk", profile.Version),
		fmt.Sprintf("/usr/lib/jvm/openjdk-%s", profile.Version),
	}

	var jvmSrc string
	for _, path := range jvmPaths {
		if _, err := os.Stat(path); err == nil {
			jvmSrc = path
			break
		}
	}

	if jvmSrc == "" {
		return fmt.Errorf("JVM installation not found for Java %s", profile.Version)
	}

	// Create destination directory
	jvmDest := filepath.Join(isolatedDir, "usr", "lib", "jvm", filepath.Base(jvmSrc))
	if err := os.MkdirAll(filepath.Dir(jvmDest), 0755); err != nil {
		return fmt.Errorf("failed to create JVM directory: %w", err)
	}

	// Copy the entire JVM directory using cp -r
	logger.Info("Copying JVM from %s", jvmSrc)
	cmd := exec.Command("cp", "-r", jvmSrc, jvmDest)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy JVM: %w\nOutput: %s", err, string(output))
	}

	logger.Debug("Copied JVM to %s", jvmDest)

	// Create symlinks in /usr/bin for java, javac, jar
	binDir := filepath.Join(isolatedDir, "usr", "bin")
	jvmBinDir := filepath.Join(jvmDest, "bin")

	javaBinaries := []string{"java", "javac", "jar", "jshell", "keytool"}
	for _, bin := range javaBinaries {
		srcBin := filepath.Join(jvmBinDir, bin)
		if _, err := os.Stat(srcBin); os.IsNotExist(err) {
			continue
		}

		destBin := filepath.Join(binDir, bin)
		os.Remove(destBin) // Remove any existing file/symlink

		// Create relative symlink
		relPath := fmt.Sprintf("../lib/jvm/%s/bin/%s", filepath.Base(jvmSrc), bin)
		if err := os.Symlink(relPath, destBin); err != nil {
			logger.Warn("Failed to create symlink for %s: %v", bin, err)
		} else {
			logger.Debug("Created symlink: %s -> %s", bin, relPath)
		}
	}

	return nil
}

// CopyLibraries copies essential and language-specific libraries to the isolated directory
func CopyLibraries(platform *PlatformInfo, profile *LanguageProfile, isolatedDir string, logger BuildLogger) error {
	libDir := filepath.Join(isolatedDir, "lib")
	lib64Dir := filepath.Join(isolatedDir, "lib64")
	usrLibDir := filepath.Join(isolatedDir, "usr", "lib")

	for _, dir := range []string{libDir, lib64Dir, usrLibDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create lib directory %s: %w", dir, err)
		}
	}

	// Collect all library patterns
	patterns := make([]string, 0)
	patterns = append(patterns, essentialLibraryPatterns...)
	if profile != nil {
		patterns = append(patterns, profile.LibraryPatterns...)
	}

	// Search directories for libraries
	searchDirs := []string{
		platform.LibPath,
		"/lib",
		"/lib64",
		"/usr/lib",
		"/usr/lib64",
	}

	copiedLibs := make(map[string]bool)

	for _, searchDir := range searchDirs {
		if _, err := os.Stat(searchDir); os.IsNotExist(err) {
			continue
		}

		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(searchDir, pattern))
			if err != nil {
				continue
			}

			for _, match := range matches {
				// Determine destination directory
				var destDir string
				if strings.Contains(searchDir, "lib64") {
					destDir = lib64Dir
				} else if strings.HasPrefix(searchDir, "/usr") {
					destDir = usrLibDir
				} else {
					destDir = libDir
				}

				destPath := filepath.Join(destDir, filepath.Base(match))

				// Skip if already copied
				if copiedLibs[destPath] {
					continue
				}

				// Resolve symlinks
				realPath, err := filepath.EvalSymlinks(match)
				if err != nil {
					realPath = match
				}

				// Copy the library
				if err := copyFile(realPath, destPath); err != nil {
					logger.Warn("Failed to copy library %s: %v", match, err)
					continue
				}

				copiedLibs[destPath] = true
				logger.Debug("Copied library: %s", filepath.Base(match))
			}
		}
	}

	return nil
}

// CopyConfiguration copies essential configuration files
func CopyConfiguration(isolatedDir string, logger BuildLogger) error {
	etcDir := filepath.Join(isolatedDir, "etc")
	sslDir := filepath.Join(etcDir, "ssl", "certs")
	tmpDir := filepath.Join(isolatedDir, "tmp")

	for _, dir := range []string{etcDir, sslDir, tmpDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Set tmp directory permissions
	if err := os.Chmod(tmpDir, 0777); err != nil {
		logger.Warn("Failed to set tmp permissions: %v", err)
	}

	// Copy essential config files
	configFiles := map[string]string{
		"/etc/resolv.conf":   filepath.Join(etcDir, "resolv.conf"),
		"/etc/hosts":         filepath.Join(etcDir, "hosts"),
		"/etc/passwd":        filepath.Join(etcDir, "passwd"),
		"/etc/group":         filepath.Join(etcDir, "group"),
		"/etc/nsswitch.conf": filepath.Join(etcDir, "nsswitch.conf"),
	}

	for src, dest := range configFiles {
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		if err := copyFile(src, dest); err != nil {
			logger.Warn("Failed to copy %s: %v", src, err)
		} else {
			logger.Debug("Copied config: %s", filepath.Base(src))
		}
	}

	// Copy SSL certificates
	sslSources := []string{
		"/etc/ssl/certs/ca-certificates.crt",
		"/etc/pki/tls/certs/ca-bundle.crt",
	}

	for _, src := range sslSources {
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		dest := filepath.Join(sslDir, filepath.Base(src))
		if err := copyFile(src, dest); err != nil {
			logger.Warn("Failed to copy SSL certs from %s: %v", src, err)
		} else {
			logger.Debug("Copied SSL certificates")
			break // Only need one
		}
	}

	return nil
}

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	destFile, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

// CopyBinariesFromPath copies binaries from a specific root path (e.g., overlay merged dir)
// instead of from the host system. This is used for isolated builds.
func CopyBinariesFromPath(rootPath string, platform *PlatformInfo, profile *LanguageProfile, isolatedDir string, logger BuildLogger) error {
	binDir := filepath.Join(isolatedDir, "usr", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Collect all binaries to copy
	binaries := make([]string, 0)
	binaries = append(binaries, essentialBinaries...)
	if profile != nil {
		binaries = append(binaries, profile.Binaries...)
	}

	// Search paths within the root
	searchPaths := []string{
		filepath.Join(rootPath, "usr", "bin"),
		filepath.Join(rootPath, "usr", "local", "bin"),
		filepath.Join(rootPath, "bin"),
	}

	// Track which versioned binaries we copied for symlink fixup
	copiedBinaries := make(map[string]bool)

	for _, binary := range binaries {
		var binaryPath string
		var found bool

		// Search for binary in the root path
		for _, searchPath := range searchPaths {
			candidatePath := filepath.Join(searchPath, binary)
			if _, err := os.Stat(candidatePath); err == nil {
				binaryPath = candidatePath
				found = true
				break
			}
		}

		if !found {
			logger.Warn("Binary not found in isolated environment, skipping: %s", binary)
			continue
		}

		// Resolve symlinks
		realPath, err := filepath.EvalSymlinks(binaryPath)
		if err != nil {
			logger.Warn("Failed to resolve symlink for %s: %v", binary, err)
			realPath = binaryPath
		}

		// Copy the binary
		destPath := filepath.Join(binDir, filepath.Base(binaryPath))
		if err := copyFile(realPath, destPath); err != nil {
			return fmt.Errorf("failed to copy binary %s: %w", binary, err)
		}

		// Track which binaries we copied
		copiedBinaries[binary] = true

		// Create symlink if original was a symlink
		if binaryPath != realPath {
			linkPath := filepath.Join(binDir, filepath.Base(binaryPath))
			realName := filepath.Base(realPath)
			os.Remove(linkPath) // Remove if exists
			if err := os.Symlink(realName, linkPath); err != nil {
				logger.Warn("Failed to create symlink for %s: %v", binary, err)
			}
		}

		logger.Debug("Copied binary from isolated env: %s", binary)
	}

	// Fix up symlinks for language runtimes
	if profile != nil {
		fixupSymlinks(binDir, profile, copiedBinaries, logger)
	}

	return nil
}

// CopyJavaRuntimeFromPath copies JVM from a specific root path (overlay)
func CopyJavaRuntimeFromPath(rootPath string, profile *LanguageProfile, isolatedDir string, logger BuildLogger) error {
	if profile == nil || profile.Language != "java" {
		return nil
	}

	// Find the JVM installation directory within the root path
	jvmPaths := []string{
		filepath.Join(rootPath, "usr", "lib", "jvm", fmt.Sprintf("java-%s-openjdk-amd64", profile.Version)),
		filepath.Join(rootPath, "usr", "lib", "jvm", fmt.Sprintf("java-%s-openjdk", profile.Version)),
		filepath.Join(rootPath, "usr", "lib", "jvm", fmt.Sprintf("openjdk-%s", profile.Version)),
	}

	var jvmSrc string
	for _, path := range jvmPaths {
		if _, err := os.Stat(path); err == nil {
			jvmSrc = path
			break
		}
	}

	if jvmSrc == "" {
		return fmt.Errorf("JVM installation not found in isolated environment for Java %s", profile.Version)
	}

	// Create destination directory
	jvmDest := filepath.Join(isolatedDir, "usr", "lib", "jvm", filepath.Base(jvmSrc))
	if err := os.MkdirAll(filepath.Dir(jvmDest), 0755); err != nil {
		return fmt.Errorf("failed to create JVM directory: %w", err)
	}

	// Copy the entire JVM directory using cp -r
	logger.Info("Copying JVM from isolated environment: %s", jvmSrc)
	cmd := exec.Command("cp", "-r", jvmSrc, jvmDest)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy JVM: %w\nOutput: %s", err, string(output))
	}

	logger.Debug("Copied JVM to %s", jvmDest)

	// Create symlinks in /usr/bin for java, javac, jar
	binDir := filepath.Join(isolatedDir, "usr", "bin")
	jvmBinDir := filepath.Join(jvmDest, "bin")

	javaBinaries := []string{"java", "javac", "jar", "jshell", "keytool"}
	for _, bin := range javaBinaries {
		srcBin := filepath.Join(jvmBinDir, bin)
		if _, err := os.Stat(srcBin); os.IsNotExist(err) {
			continue
		}

		destBin := filepath.Join(binDir, bin)
		os.Remove(destBin) // Remove any existing file/symlink

		// Create relative symlink
		relPath := fmt.Sprintf("../lib/jvm/%s/bin/%s", filepath.Base(jvmSrc), bin)
		if err := os.Symlink(relPath, destBin); err != nil {
			logger.Warn("Failed to create symlink for %s: %v", bin, err)
		} else {
			logger.Debug("Created symlink: %s -> %s", bin, relPath)
		}
	}

	return nil
}

// CopyPythonRuntimeFromPath copies the Python standard library and site-packages from an overlay
func CopyPythonRuntimeFromPath(rootPath string, profile *LanguageProfile, isolatedDir string, logger BuildLogger) error {
	if profile == nil || profile.Language != "python" {
		return nil
	}

	// Find the Python stdlib directory within the root path
	pythonVersion := fmt.Sprintf("python%s", profile.Version)
	stdlibPaths := []string{
		filepath.Join(rootPath, "usr", "lib", pythonVersion),
		filepath.Join(rootPath, "usr", "lib", "python3"),
	}

	var stdlibSrc string
	for _, path := range stdlibPaths {
		if _, err := os.Stat(path); err == nil {
			stdlibSrc = path
			break
		}
	}

	if stdlibSrc == "" {
		return fmt.Errorf("python stdlib not found in isolated environment for Python %s", profile.Version)
	}

	// Create destination directory
	stdlibDest := filepath.Join(isolatedDir, "usr", "lib", filepath.Base(stdlibSrc))
	if err := os.MkdirAll(filepath.Dir(stdlibDest), 0755); err != nil {
		return fmt.Errorf("failed to create Python lib directory: %w", err)
	}

	// Copy the entire Python stdlib using cp -r
	logger.Info("Copying Python stdlib from isolated environment: %s", stdlibSrc)
	cmd := exec.Command("cp", "-r", stdlibSrc, stdlibDest)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy Python stdlib: %w\nOutput: %s", err, string(output))
	}

	logger.Debug("Copied Python stdlib to %s", stdlibDest)

	// Also copy site-packages from /usr/local/lib/pythonX.Y/
	localSitePackages := filepath.Join(rootPath, "usr", "local", "lib", pythonVersion, "dist-packages")
	if _, err := os.Stat(localSitePackages); err == nil {
		destSitePackages := filepath.Join(isolatedDir, "usr", "local", "lib", pythonVersion, "dist-packages")
		if err := os.MkdirAll(filepath.Dir(destSitePackages), 0755); err != nil {
			return fmt.Errorf("failed to create site-packages directory: %w", err)
		}

		logger.Info("Copying Python site-packages from isolated environment")
		cmd := exec.Command("cp", "-r", localSitePackages, destSitePackages)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Warn("Failed to copy site-packages: %v\nOutput: %s", err, string(output))
		} else {
			logger.Debug("Copied site-packages to %s", destSitePackages)
		}
	}

	// Also check for pip-installed packages in /usr/lib/python3/dist-packages
	python3DistPackages := filepath.Join(rootPath, "usr", "lib", "python3", "dist-packages")
	if _, err := os.Stat(python3DistPackages); err == nil {
		destDistPackages := filepath.Join(isolatedDir, "usr", "lib", "python3", "dist-packages")
		if err := os.MkdirAll(filepath.Dir(destDistPackages), 0755); err != nil {
			return fmt.Errorf("failed to create dist-packages directory: %w", err)
		}

		logger.Info("Copying Python dist-packages from isolated environment")
		cmd := exec.Command("cp", "-r", python3DistPackages, destDistPackages)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Warn("Failed to copy dist-packages: %v\nOutput: %s", err, string(output))
		} else {
			logger.Debug("Copied dist-packages to %s", destDistPackages)
		}
	}

	return nil
}

// CopyLibrariesFromPath copies libraries from a specific root path (overlay)
// extraPatterns allows runtime.yaml to specify additional library patterns
func CopyLibrariesFromPath(rootPath string, platform *PlatformInfo, profile *LanguageProfile, isolatedDir string, extraPatterns []string, logger BuildLogger) error {
	libDir := filepath.Join(isolatedDir, "lib")
	lib64Dir := filepath.Join(isolatedDir, "lib64")
	usrLibDir := filepath.Join(isolatedDir, "usr", "lib")

	for _, dir := range []string{libDir, lib64Dir, usrLibDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create lib directory %s: %w", dir, err)
		}
	}

	// Collect all library patterns:
	// 1. Essential patterns (libc, libm, etc.)
	// 2. Language profile patterns (libpython*, libssl*, etc.)
	// 3. Extra patterns from runtime.yaml (user-specified)
	patterns := make([]string, 0)
	patterns = append(patterns, essentialLibraryPatterns...)
	if profile != nil {
		patterns = append(patterns, profile.LibraryPatterns...)
	}
	patterns = append(patterns, extraPatterns...)

	// Search directories within the root path
	// Determine lib path based on platform
	var platformLibPath string
	if platform.Arch == "amd64" {
		platformLibPath = filepath.Join(rootPath, "lib", "x86_64-linux-gnu")
	} else {
		platformLibPath = filepath.Join(rootPath, "lib", "aarch64-linux-gnu")
	}

	searchDirs := []string{
		platformLibPath,
		filepath.Join(rootPath, "lib"),
		filepath.Join(rootPath, "lib64"),
		filepath.Join(rootPath, "usr", "lib"),
		filepath.Join(rootPath, "usr", "lib64"),
	}

	copiedLibs := make(map[string]bool)

	for _, searchDir := range searchDirs {
		if _, err := os.Stat(searchDir); os.IsNotExist(err) {
			continue
		}

		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(searchDir, pattern))
			if err != nil {
				continue
			}

			for _, match := range matches {
				// Determine destination directory
				var destDir string
				if strings.Contains(searchDir, "lib64") {
					destDir = lib64Dir
				} else if strings.Contains(searchDir, "usr") {
					destDir = usrLibDir
				} else {
					destDir = libDir
				}

				destPath := filepath.Join(destDir, filepath.Base(match))

				// Skip if already copied
				if copiedLibs[destPath] {
					continue
				}

				// Resolve symlinks
				realPath, err := filepath.EvalSymlinks(match)
				if err != nil {
					realPath = match
				}

				// Copy the library
				if err := copyFile(realPath, destPath); err != nil {
					logger.Warn("Failed to copy library %s: %v", match, err)
					continue
				}

				copiedLibs[destPath] = true
				logger.Debug("Copied library from isolated env: %s", filepath.Base(match))
			}
		}
	}

	return nil
}
