package platform

import (
	"os"
	"syscall"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// Platform provides a unified interface for all platform-specific operations
//
//counterfeiter:generate . Platform
type Platform interface {
	OSOperations
	SyscallOperations
	CommandFactory
	ExecOperations
}

// OSOperations defines file system and OS-level operations
//
//counterfeiter:generate . OSOperations
type OSOperations interface {
	// File operations
	WriteFile(name string, data []byte, perm os.FileMode) error
	ReadFile(path string) ([]byte, error)
	Remove(path string) error
	Symlink(source string, path string) error
	MkdirAll(dir string, perm os.FileMode) error

	// File info operations
	Stat(name string) (os.FileInfo, error)
	IsNotExist(err error) bool

	// Process info
	Executable() (string, error)
	Getpid() int
	Exit(code int)

	// Environment
	Environ() []string
	Getenv(key string) string
}

// SyscallOperations defines low-level system call operations
//
//counterfeiter:generate . SyscallOperations
type SyscallOperations interface {
	// Process control
	Kill(pid int, sig syscall.Signal) error
	Exec(argv0 string, argv []string, envv []string) error
	CreateProcessGroup() *syscall.SysProcAttr

	// Mount operations (Linux-specific, no-op on other platforms)
	Mount(source string, target string, fstype string, flags uintptr, data string) error
	Unmount(target string, flags int) error
}

// CommandFactory creates and manages command execution
//
//counterfeiter:generate . CommandFactory
type CommandFactory interface {
	CreateCommand(name string, args ...string) Command
}

// Command represents an executing command
//
//counterfeiter:generate . Command
type Command interface {
	Start() error
	Wait() error
	Process() Process
	SetStdout(w interface{})
	SetStderr(w interface{})
	SetSysProcAttr(attr *syscall.SysProcAttr)
	SetEnv(env []string)
}

// Process represents a running process
//
//counterfeiter:generate . Process
type Process interface {
	Pid() int
	Kill() error
}

// ExecOperations defines executable resolution operations
//
//counterfeiter:generate . ExecOperations
type ExecOperations interface {
	LookPath(file string) (string, error)
}

// Info provides information about the current platform
type Info struct {
	OS           string
	Architecture string
}
