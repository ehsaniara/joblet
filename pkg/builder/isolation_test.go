//go:build linux

package builder_test

import (
	"errors"
	"os"
	"testing"

	"github.com/ehsaniara/joblet/pkg/builder"
	"github.com/ehsaniara/joblet/pkg/builder/builderfakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIsolatedEnvironmentWithOps(t *testing.T) {
	t.Run("creates environment successfully", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}
		logger := builder.NewBuildLogger(false)

		env, err := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

		require.NoError(t, err)
		assert.NotNil(t, env)
		// Verify paths via exported methods
		assert.Equal(t, "/tmp/test-base/merged/usr/bin", env.GetMergedPath("/usr/bin"))
		assert.Equal(t, "/tmp/test-base/upper/usr/lib", env.GetUpperPath("/usr/lib"))
		assert.False(t, env.IsMounted())
	})

	t.Run("creates default logger if nil", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}

		env, err := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", nil, fakeSysOps)

		require.NoError(t, err)
		assert.NotNil(t, env)
	})

	t.Run("creates default sysOps if nil", func(t *testing.T) {
		logger := builder.NewBuildLogger(false)

		env, err := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, nil)

		require.NoError(t, err)
		assert.NotNil(t, env)
	})
}

func TestIsolatedEnvironment_Setup(t *testing.T) {
	t.Run("creates directories and mounts overlay", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}
		fakeCmdRunner := &builderfakes.FakeCmdRunner{}
		logger := builder.NewBuildLogger(false)
		env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

		// Mock Stat to return "not found" for resolv.conf (so it tries to create it)
		fakeSysOps.StatReturns(nil, os.ErrNotExist)
		// Mock ReadFile for /etc/resolv.conf
		fakeSysOps.ReadFileReturns([]byte("nameserver 8.8.8.8\n"), nil)
		// Mock Command for DNS verification (getent hosts google.com)
		fakeSysOps.CommandReturns(fakeCmdRunner)
		fakeCmdRunner.CombinedOutputReturns([]byte("142.250.80.46 google.com"), nil)

		err := env.Setup()

		require.NoError(t, err)
		assert.True(t, env.IsMounted())

		// Verify MkdirAll was called for upper, work, merged directories
		mkdirCalls := fakeSysOps.MkdirAllCallCount()
		assert.GreaterOrEqual(t, mkdirCalls, 3)

		// Verify Mount was called for overlay, proc, sys, dev
		mountCalls := fakeSysOps.MountCallCount()
		assert.GreaterOrEqual(t, mountCalls, 4)

		// Check overlay mount
		source, target, fstype, _, data := fakeSysOps.MountArgsForCall(0)
		assert.Equal(t, "overlay", source)
		assert.Equal(t, "/tmp/test-base/merged", target)
		assert.Equal(t, "overlay", fstype)
		assert.Contains(t, data, "lowerdir=/")
		assert.Contains(t, data, "upperdir=/tmp/test-base/upper")
	})

	t.Run("returns error on MkdirAll failure", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}
		logger := builder.NewBuildLogger(false)
		env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

		fakeSysOps.MkdirAllReturns(errors.New("permission denied"))

		err := env.Setup()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create directory")
		assert.False(t, env.IsMounted())
	})

	t.Run("returns error on Mount failure", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}
		logger := builder.NewBuildLogger(false)
		env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

		// MkdirAll succeeds, but Mount fails
		fakeSysOps.MountReturns(errors.New("operation not permitted"))

		err := env.Setup()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to mount overlayfs")
		assert.False(t, env.IsMounted())
	})
}

func TestIsolatedEnvironment_RunInChroot(t *testing.T) {
	t.Run("returns error when not mounted", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}
		logger := builder.NewBuildLogger(false)
		env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

		output, err := env.RunInChroot("echo", "hello")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not mounted")
		assert.Nil(t, output)
	})

	t.Run("executes command in chroot when mounted", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}
		fakeCmdRunner := &builderfakes.FakeCmdRunner{}
		logger := builder.NewBuildLogger(false)
		env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

		// Setup the environment first (to set mounted = true)
		fakeSysOps.StatReturns(nil, os.ErrNotExist)
		fakeSysOps.ReadFileReturns([]byte("nameserver 8.8.8.8\n"), nil)
		// Mock Command for DNS verification during Setup
		fakeSysOps.CommandReturns(fakeCmdRunner)
		fakeCmdRunner.CombinedOutputReturns([]byte("142.250.80.46 google.com"), nil)
		_ = env.Setup()

		// Reset command stub for the actual test
		fakeSysOps.CommandReturns(fakeCmdRunner)
		fakeCmdRunner.CombinedOutputReturns([]byte("command output"), nil)

		output, err := env.RunInChroot("echo", "hello")

		require.NoError(t, err)
		assert.Equal(t, []byte("command output"), output)

		// Verify Command was called with chroot (find the call with "echo")
		foundCall := false
		for i := 0; i < fakeSysOps.CommandCallCount(); i++ {
			name, args := fakeSysOps.CommandArgsForCall(i)
			if name == "chroot" && len(args) >= 2 && args[1] == "echo" {
				foundCall = true
				assert.Equal(t, "/tmp/test-base/merged", args[0])
				assert.Equal(t, "hello", args[2])
				break
			}
		}
		assert.True(t, foundCall, "Expected chroot command call not found")
	})
}

func TestIsolatedEnvironment_InstallPackagesIsolated(t *testing.T) {
	t.Run("skips when no packages", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}
		fakeCmdRunner := &builderfakes.FakeCmdRunner{}
		logger := builder.NewBuildLogger(false)
		env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

		// Setup the environment first
		fakeSysOps.StatReturns(nil, os.ErrNotExist)
		fakeSysOps.ReadFileReturns([]byte("nameserver 8.8.8.8\n"), nil)
		fakeSysOps.CommandReturns(fakeCmdRunner)
		fakeCmdRunner.CombinedOutputReturns([]byte("142.250.80.46 google.com"), nil)
		_ = env.Setup()

		initialCalls := fakeSysOps.CommandCallCount()

		err := env.InstallPackagesIsolated("apt", []string{})

		require.NoError(t, err)
		// No additional commands should be called
		assert.Equal(t, initialCalls, fakeSysOps.CommandCallCount())
	})

	t.Run("installs apt packages", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}
		fakeCmdRunner := &builderfakes.FakeCmdRunner{}
		logger := builder.NewBuildLogger(false)
		env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

		// Setup the environment first
		fakeSysOps.StatReturns(nil, os.ErrNotExist)
		fakeSysOps.ReadFileReturns([]byte("nameserver 8.8.8.8\n"), nil)
		fakeSysOps.CommandReturns(fakeCmdRunner)
		fakeCmdRunner.CombinedOutputReturns([]byte("142.250.80.46 google.com"), nil)
		_ = env.Setup()

		initialCalls := fakeSysOps.CommandCallCount()
		fakeSysOps.CommandReturns(fakeCmdRunner)
		fakeCmdRunner.CombinedOutputReturns([]byte("installed"), nil)

		err := env.InstallPackagesIsolated("apt", []string{"curl", "wget"})

		require.NoError(t, err)

		// Should call apt-get update and apt-get install (2 new commands)
		newCalls := fakeSysOps.CommandCallCount() - initialCalls
		assert.Equal(t, 2, newCalls)
	})

	t.Run("returns error on unsupported package manager", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}
		fakeCmdRunner := &builderfakes.FakeCmdRunner{}
		logger := builder.NewBuildLogger(false)
		env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

		// Setup the environment first
		fakeSysOps.StatReturns(nil, os.ErrNotExist)
		fakeSysOps.ReadFileReturns([]byte("nameserver 8.8.8.8\n"), nil)
		fakeSysOps.CommandReturns(fakeCmdRunner)
		fakeCmdRunner.CombinedOutputReturns([]byte("142.250.80.46 google.com"), nil)
		_ = env.Setup()

		err := env.InstallPackagesIsolated("pacman", []string{"curl"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported package manager")
	})
}

func TestIsolatedEnvironment_Cleanup(t *testing.T) {
	t.Run("unmounts filesystems in reverse order", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}
		fakeCmdRunner := &builderfakes.FakeCmdRunner{}
		logger := builder.NewBuildLogger(false)
		env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

		// Setup the environment first to set mounted = true
		fakeSysOps.StatReturns(nil, os.ErrNotExist)
		fakeSysOps.ReadFileReturns([]byte("nameserver 8.8.8.8\n"), nil)
		fakeSysOps.CommandReturns(fakeCmdRunner)
		fakeCmdRunner.CombinedOutputReturns([]byte("142.250.80.46 google.com"), nil)
		_ = env.Setup()

		assert.True(t, env.IsMounted())

		err := env.Cleanup()

		require.NoError(t, err)
		assert.False(t, env.IsMounted())

		// Verify Unmount was called for dev, sys, proc, and overlay
		unmountCalls := fakeSysOps.UnmountCallCount()
		assert.Equal(t, 4, unmountCalls)

		// Check order: dev, sys, proc, overlay
		target, _ := fakeSysOps.UnmountArgsForCall(0)
		assert.Equal(t, "/tmp/test-base/merged/dev", target)

		target, _ = fakeSysOps.UnmountArgsForCall(1)
		assert.Equal(t, "/tmp/test-base/merged/sys", target)

		target, _ = fakeSysOps.UnmountArgsForCall(2)
		assert.Equal(t, "/tmp/test-base/merged/proc", target)

		target, _ = fakeSysOps.UnmountArgsForCall(3)
		assert.Equal(t, "/tmp/test-base/merged", target)

		// Verify RemoveAll was called
		assert.Equal(t, 1, fakeSysOps.RemoveAllCallCount())
		path := fakeSysOps.RemoveAllArgsForCall(0)
		assert.Equal(t, "/tmp/test-base", path)
	})

	t.Run("skips unmount when not mounted", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}
		logger := builder.NewBuildLogger(false)
		env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)
		// Don't call Setup, so mounted = false

		err := env.Cleanup()

		require.NoError(t, err)
		assert.Equal(t, 0, fakeSysOps.UnmountCallCount())
		// RemoveAll should still be called
		assert.Equal(t, 1, fakeSysOps.RemoveAllCallCount())
	})

	t.Run("collects errors but continues cleanup", func(t *testing.T) {
		fakeSysOps := &builderfakes.FakeSystemOps{}
		fakeCmdRunner := &builderfakes.FakeCmdRunner{}
		logger := builder.NewBuildLogger(false)
		env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

		// Setup the environment first
		fakeSysOps.StatReturns(nil, os.ErrNotExist)
		fakeSysOps.ReadFileReturns([]byte("nameserver 8.8.8.8\n"), nil)
		fakeSysOps.CommandReturns(fakeCmdRunner)
		fakeCmdRunner.CombinedOutputReturns([]byte("142.250.80.46 google.com"), nil)
		_ = env.Setup()

		fakeSysOps.UnmountReturns(errors.New("device busy"))
		fakeSysOps.RemoveAllReturns(errors.New("permission denied"))

		err := env.Cleanup()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cleanup errors")
		// Should still attempt all unmounts and removeall
		assert.Equal(t, 4, fakeSysOps.UnmountCallCount())
		assert.Equal(t, 1, fakeSysOps.RemoveAllCallCount())
	})
}

func TestIsolatedEnvironment_GetPaths(t *testing.T) {
	fakeSysOps := &builderfakes.FakeSystemOps{}
	logger := builder.NewBuildLogger(false)
	env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

	t.Run("GetMergedPath returns correct path", func(t *testing.T) {
		path := env.GetMergedPath("/usr/bin/python")
		assert.Equal(t, "/tmp/test-base/merged/usr/bin/python", path)
	})

	t.Run("GetUpperPath returns correct path", func(t *testing.T) {
		path := env.GetUpperPath("/usr/lib/libfoo.so")
		assert.Equal(t, "/tmp/test-base/upper/usr/lib/libfoo.so", path)
	})
}

func TestIsolatedEnvironment_IsMounted(t *testing.T) {
	fakeSysOps := &builderfakes.FakeSystemOps{}
	fakeCmdRunner := &builderfakes.FakeCmdRunner{}
	logger := builder.NewBuildLogger(false)
	env, _ := builder.NewIsolatedEnvironmentWithOps("/tmp/test-base", logger, fakeSysOps)

	assert.False(t, env.IsMounted())

	// Setup to make it mounted
	fakeSysOps.StatReturns(nil, os.ErrNotExist)
	fakeSysOps.ReadFileReturns([]byte("nameserver 8.8.8.8\n"), nil)
	fakeSysOps.CommandReturns(fakeCmdRunner)
	fakeCmdRunner.CombinedOutputReturns([]byte("142.250.80.46 google.com"), nil)
	_ = env.Setup()

	assert.True(t, env.IsMounted())
}
