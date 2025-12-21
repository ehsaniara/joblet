//go:build linux

package builder

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// SystemOps defines the interface for system operations used by the isolated builder.
// This interface allows for mocking in unit tests.
//
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . SystemOps
type SystemOps interface {
	// File system operations
	MkdirAll(path string, perm os.FileMode) error
	Stat(path string) (fs.FileInfo, error)
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	RemoveAll(path string) error
	Open(path string) (*os.File, error)
	OpenFile(path string, flag int, perm os.FileMode) (*os.File, error)
	Glob(pattern string) ([]string, error)

	// Mount operations
	Mount(source string, target string, fstype string, flags uintptr, data string) error
	Unmount(target string, flags int) error

	// Command execution
	Command(name string, args ...string) CmdRunner
}

// CmdRunner defines the interface for command execution.
// This allows mocking exec.Cmd behavior.
//
//counterfeiter:generate . CmdRunner
type CmdRunner interface {
	SetEnv(env []string)
	CombinedOutput() ([]byte, error)
}

// RealSystemOps implements SystemOps using real system calls
type RealSystemOps struct{}

// NewRealSystemOps creates a new RealSystemOps instance
func NewRealSystemOps() *RealSystemOps {
	return &RealSystemOps{}
}

// MkdirAll creates a directory and all parent directories
func (r *RealSystemOps) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Stat returns file info for the given path
func (r *RealSystemOps) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

// ReadFile reads the entire file contents
func (r *RealSystemOps) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes data to a file
func (r *RealSystemOps) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// RemoveAll removes a directory and all its contents
func (r *RealSystemOps) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Open opens a file for reading
func (r *RealSystemOps) Open(path string) (*os.File, error) {
	return os.Open(path)
}

// OpenFile opens a file with the specified flags and permissions
func (r *RealSystemOps) OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag, perm)
}

// Glob returns file paths matching the pattern
func (r *RealSystemOps) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

// Mount mounts a filesystem
func (r *RealSystemOps) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return syscall.Mount(source, target, fstype, flags, data)
}

// Unmount unmounts a filesystem
func (r *RealSystemOps) Unmount(target string, flags int) error {
	return syscall.Unmount(target, flags)
}

// Command creates a new command runner
func (r *RealSystemOps) Command(name string, args ...string) CmdRunner {
	return &RealCmdRunner{cmd: exec.Command(name, args...)}
}

// RealCmdRunner wraps exec.Cmd to implement CmdRunner
type RealCmdRunner struct {
	cmd *exec.Cmd
}

// SetEnv sets the environment variables for the command
func (r *RealCmdRunner) SetEnv(env []string) {
	r.cmd.Env = env
}

// CombinedOutput runs the command and returns combined stdout and stderr
func (r *RealCmdRunner) CombinedOutput() ([]byte, error) {
	return r.cmd.CombinedOutput()
}

// Helper function to copy file contents (used by isolation)
func CopyFileContents(sysOps SystemOps, src, dest string, mode os.FileMode) error {
	srcFile, err := sysOps.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := sysOps.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}
