package platform

import (
	"os"
	"syscall"
)

// MockPlatform provides a mock implementation for testing
type MockPlatform struct {
	*BasePlatform

	// Mock behavior flags
	ShouldFailMount bool
	ShouldFailKill  bool
	ShouldFailExec  bool

	// Call tracking
	MountCalls []MountCall
	KillCalls  []KillCall
	ExecCalls  []ExecCall
}

type MountCall struct {
	Source string
	Target string
	FSType string
	Flags  uintptr
	Data   string
}

type KillCall struct {
	PID    int
	Signal syscall.Signal
}

type ExecCall struct {
	Argv0 string
	Argv  []string
	Envv  []string
}

// NewMockPlatform creates a new mock platform for testing
func NewMockPlatform() *MockPlatform {
	return &MockPlatform{
		BasePlatform: NewBasePlatform(),
		MountCalls:   make([]MountCall, 0),
		KillCalls:    make([]KillCall, 0),
		ExecCalls:    make([]ExecCall, 0),
	}
}

// Mock implementations that track calls
func (mp *MockPlatform) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	mp.MountCalls = append(mp.MountCalls, MountCall{
		Source: source,
		Target: target,
		FSType: fstype,
		Flags:  flags,
		Data:   data,
	})

	if mp.ShouldFailMount {
		return NewPlatformError("mock", "mount", os.ErrPermission)
	}

	return nil
}

func (mp *MockPlatform) Kill(pid int, sig syscall.Signal) error {
	mp.KillCalls = append(mp.KillCalls, KillCall{
		PID:    pid,
		Signal: sig,
	})

	if mp.ShouldFailKill {
		return NewPlatformError("mock", "kill", syscall.ESRCH)
	}

	return nil
}

func (mp *MockPlatform) Exec(argv0 string, argv []string, envv []string) error {
	mp.ExecCalls = append(mp.ExecCalls, ExecCall{
		Argv0: argv0,
		Argv:  argv,
		Envv:  envv,
	})

	if mp.ShouldFailExec {
		return NewPlatformError("mock", "exec", os.ErrNotExist)
	}

	return nil
}

func (mp *MockPlatform) GetInfo() *Info {
	return &Info{
		OS:           "mock",
		Architecture: "mock",
	}
}

func (mp *MockPlatform) ValidateRequirements() error {
	return nil // Always pass in tests
}

// Reset clears all call tracking
func (mp *MockPlatform) Reset() {
	mp.MountCalls = mp.MountCalls[:0]
	mp.KillCalls = mp.KillCalls[:0]
	mp.ExecCalls = mp.ExecCalls[:0]
	mp.ShouldFailMount = false
	mp.ShouldFailKill = false
	mp.ShouldFailExec = false
}
