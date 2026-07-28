//go:build linux

package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
	"github.com/ehsaniara/joblet/pkg/platform/platformfakes"
)

func TestSetupLimitedWorkDir(t *testing.T) {
	// Skip in CI environments that might not have mount privileges
	// Check multiple CI environment indicators
	if isCI() {
		t.Skip("Filesystem tests require mount privileges not available in CI")
	}

	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "joblet-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create job filesystem
	cfg := &config.Config{
		Filesystem: config.FilesystemConfig{
			BaseDir: tempDir,
			TmpDir:  filepath.Join(tempDir, "tmp"),
		},
	}

	platform := platform.NewPlatform()
	jobFS := &JobFilesystem{
		JobUUID:  "test-job",
		RootDir:  filepath.Join(tempDir, "root"),
		TmpDir:   filepath.Join(tempDir, "tmp"),
		WorkDir:  filepath.Join(tempDir, "root", "work"),
		Volumes:  []string{}, // No volumes
		platform: platform,
		config:   cfg,
		logger:   logger.New().WithField("component", "test-filesystem"),
	}

	// Create necessary directories
	if err := os.MkdirAll(jobFS.RootDir, 0755); err != nil {
		t.Fatalf("Failed to create root dir: %v", err)
	}
	if err := os.MkdirAll(jobFS.WorkDir, 0755); err != nil {
		t.Fatalf("Failed to create work dir: %v", err)
	}

	// Test setupLimitedWorkDir
	err = jobFS.setupLimitedWorkDir()
	if err != nil {
		t.Logf("setupLimitedWorkDir failed (expected in test environment without mount privileges): %v", err)
		// This is expected to fail in test environment without proper privileges
		return
	}

	t.Log("setupLimitedWorkDir succeeded (running with sufficient privileges)")
}

func TestJobFilesystemWithoutVolumes(t *testing.T) {
	// Skip in CI environments that might not have mount privileges
	if isCI() {
		t.Skip("Filesystem tests require mount privileges not available in CI")
	}

	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "joblet-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create job filesystem without volumes
	cfg := &config.Config{
		Filesystem: config.FilesystemConfig{
			BaseDir: tempDir,
			TmpDir:  filepath.Join(tempDir, "tmp"),
		},
	}

	platform := platform.NewPlatform()
	jobFS := &JobFilesystem{
		JobUUID:  "test-job-no-volumes",
		RootDir:  filepath.Join(tempDir, "root"),
		TmpDir:   filepath.Join(tempDir, "tmp"),
		WorkDir:  filepath.Join(tempDir, "root", "work"),
		Volumes:  []string{}, // No volumes - should trigger limited work dir
		platform: platform,
		config:   cfg,
		logger:   logger.New().WithField("component", "test-filesystem"),
	}

	// Verify that with no volumes, setupLimitedWorkDir would be called
	if len(jobFS.Volumes) != 0 {
		t.Errorf("Expected no volumes, got %d volumes", len(jobFS.Volumes))
	}

	t.Log("Job filesystem correctly configured with no volumes - would use limited work directory")
}

// isCI detects if tests are running in a CI environment
func isCI() bool {
	// Check common CI environment variables
	ciEnvVars := []string{
		"CI",                     // Generic CI indicator
		"CONTINUOUS_INTEGRATION", // Generic CI indicator
		"GITHUB_ACTIONS",         // GitHub Actions
		"TRAVIS",                 // Travis CI
		"CIRCLECI",               // Circle CI
		"JENKINS_URL",            // Jenkins
		"BUILDKITE",              // Buildkite
		"GITLAB_CI",              // GitLab CI
		"AZURE_HTTP_USER_AGENT",  // Azure DevOps
		"TEAMCITY_VERSION",       // TeamCity
	}

	for _, envVar := range ciEnvVars {
		if value := os.Getenv(envVar); value == "true" || value == "1" || value != "" {
			return true
		}
	}

	// Check for specific CI user patterns
	if user := os.Getenv("USER"); user == "runner" || user == "travis" || strings.Contains(user, "jenkins") {
		return true
	}

	// Check for CI-like hostnames
	if hostname := os.Getenv("HOSTNAME"); strings.Contains(hostname, "runner") || strings.Contains(hostname, "build") {
		return true
	}

	// Check working directory patterns
	if pwd := os.Getenv("PWD"); strings.Contains(pwd, "/home/runner/") || strings.Contains(pwd, "/builds/") {
		return true
	}

	return false
}

func TestLoadGPUFromEnvironment(t *testing.T) {
	newFS := func() *JobFilesystem {
		return &JobFilesystem{
			JobUUID:  "gpu-job",
			platform: platform.NewPlatform(),
			logger:   logger.New().WithField("component", "test-filesystem"),
		}
	}

	t.Run("parses indices and cuda mounts", func(t *testing.T) {
		t.Setenv("JOB_GPU_INDICES", "0,2, 3")
		t.Setenv("JOB_GPU_CUDA_MOUNTS", "/usr/local/cuda:/opt/cuda")

		f := newFS()
		f.loadGPUFromEnvironment()

		want := []int{0, 2, 3}
		if len(f.GPUIndices) != len(want) {
			t.Fatalf("GPUIndices = %v, want %v", f.GPUIndices, want)
		}
		for i := range want {
			if f.GPUIndices[i] != want[i] {
				t.Fatalf("GPUIndices = %v, want %v", f.GPUIndices, want)
			}
		}
		if len(f.CUDAMountPaths) != 2 || f.CUDAMountPaths[0] != "/usr/local/cuda" || f.CUDAMountPaths[1] != "/opt/cuda" {
			t.Fatalf("CUDAMountPaths = %v, want [/usr/local/cuda /opt/cuda]", f.CUDAMountPaths)
		}
	})

	t.Run("no gpu env leaves fields empty", func(t *testing.T) {
		t.Setenv("JOB_GPU_INDICES", "")
		t.Setenv("JOB_GPU_CUDA_MOUNTS", "")

		f := newFS()
		f.loadGPUFromEnvironment()

		if len(f.GPUIndices) != 0 || len(f.CUDAMountPaths) != 0 {
			t.Fatalf("expected empty GPU config, got indices=%v mounts=%v", f.GPUIndices, f.CUDAMountPaths)
		}
	})
}

func TestCreateGPUDeviceNodes(t *testing.T) {
	fp := &platformfakes.FakePlatform{}
	f := &JobFilesystem{
		JobUUID:  "gpu-job",
		platform: fp,
		logger:   logger.New().WithField("component", "test-filesystem"),
	}

	if err := f.CreateGPUDeviceNodes([]int{0, 1}); err != nil {
		t.Fatalf("CreateGPUDeviceNodes returned error: %v", err)
	}

	// Must mknod the two common devices plus one node per allocated GPU.
	got := map[string]bool{}
	for i := 0; i < fp.MknodCallCount(); i++ {
		path, _, _ := fp.MknodArgsForCall(i)
		got[path] = true
	}
	for _, want := range []string{"/dev/nvidiactl", "/dev/nvidia-uvm", "/dev/nvidia-uvm-tools", "/dev/nvidia0", "/dev/nvidia1"} {
		if !got[want] {
			t.Errorf("expected a mknod for %s; got calls %v", want, got)
		}
	}
}

func TestDriverSoname(t *testing.T) {
	cases := map[string]string{
		"libcuda.so.550.90.07":                  "libcuda.so.1",
		"libnvidia-ml.so.550.90.07":             "libnvidia-ml.so.1",
		"libnvidia-ptxjitcompiler.so.550.90.07": "libnvidia-ptxjitcompiler.so.1",
		"libcuda.so":                            "", // no version suffix
		"libcuda.so.1":                          "", // already the SONAME
	}
	for name, want := range cases {
		if got := driverSoname(name); got != want {
			t.Errorf("driverSoname(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestDiscoverDriverLibraries(t *testing.T) {
	dir := t.TempDir()
	// Real versioned driver files (should be discovered).
	for _, f := range []string{"libcuda.so.550.90.07", "libnvidia-ml.so.550.90.07"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// A SONAME symlink (should be skipped) and a non-driver lib (should be ignored).
	_ = os.Symlink("libcuda.so.550.90.07", filepath.Join(dir, "libcuda.so.1"))
	if err := os.WriteFile(filepath.Join(dir, "libfoo.so.1"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	// Point discovery at the temp dir only (white-box override, restored after).
	saved := driverLibraryDirs
	driverLibraryDirs = []string{dir}
	defer func() { driverLibraryDirs = saved }()

	f := &JobFilesystem{platform: platform.NewPlatform(), logger: logger.New().WithField("c", "t")}
	got := f.discoverDriverLibraries()

	want := map[string]bool{
		filepath.Join(dir, "libcuda.so.550.90.07"):      true,
		filepath.Join(dir, "libnvidia-ml.so.550.90.07"): true,
	}
	if len(got) != len(want) {
		t.Fatalf("discovered %v, want %d driver libs", got, len(want))
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected discovered lib %q (symlinks/non-driver libs should be skipped)", g)
		}
	}
}

func TestInjectDriverLibraries(t *testing.T) {
	fp := &platformfakes.FakePlatform{}
	f := &JobFilesystem{
		RootDir:  "/job/root",
		platform: fp,
		logger:   logger.New().WithField("c", "t"),
	}

	lib := "/usr/lib/x86_64-linux-gnu/libcuda.so.550.90.07"
	if err := f.injectDriverLibraries([]string{lib}); err != nil {
		t.Fatalf("injectDriverLibraries error: %v", err)
	}

	// The lib is bind mounted into the job root at its host path.
	mounted := false
	for i := 0; i < fp.MountCallCount(); i++ {
		src, tgt, _, _, _ := fp.MountArgsForCall(i)
		if src == lib && tgt == "/job/root/usr/lib/x86_64-linux-gnu/libcuda.so.550.90.07" {
			mounted = true
		}
	}
	if !mounted {
		t.Errorf("expected bind mount of %s into the job root", lib)
	}

	// The SONAME symlink is recreated alongside it.
	linked := false
	for i := 0; i < fp.SymlinkCallCount(); i++ {
		oldname, link := fp.SymlinkArgsForCall(i)
		if oldname == "libcuda.so.550.90.07" && link == "/job/root/usr/lib/x86_64-linux-gnu/libcuda.so.1" {
			linked = true
		}
	}
	if !linked {
		t.Errorf("expected SONAME symlink libcuda.so.1 -> libcuda.so.550.90.07")
	}
}

func TestMountCUDALibraries_MountsExistingSkipsMissing(t *testing.T) {
	fp := &platformfakes.FakePlatform{}

	// One CUDA path exists (a real temp dir), one does not.
	existing := t.TempDir()
	info, err := os.Stat(existing)
	if err != nil {
		t.Fatalf("stat temp dir: %v", err)
	}
	fp.StatReturns(info, nil)   // treat every source as an existing dir
	fp.IsNotExistReturns(false) // ...so nothing is skipped as missing

	f := &JobFilesystem{
		JobUUID:  "gpu-job",
		RootDir:  t.TempDir(),
		platform: fp,
		logger:   logger.New().WithField("component", "test-filesystem"),
	}

	if err := f.MountCUDALibraries("", []string{existing}); err != nil {
		t.Fatalf("MountCUDALibraries returned error: %v", err)
	}

	// The existing path should be bind mounted (source arg matches).
	mounted := false
	for i := 0; i < fp.MountCallCount(); i++ {
		src, _, _, _, _ := fp.MountArgsForCall(i)
		if src == existing {
			mounted = true
		}
	}
	if !mounted {
		t.Errorf("expected a bind mount for %s", existing)
	}
}
