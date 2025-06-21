//go:build darwin

package os

import (
	"io"
	"os"
	"os/exec"
	"syscall"
)

// DefaultCommandFactory implements interfaces.CommandFactory using exec.Cmd
type DefaultCommandFactory struct{}

func (f *DefaultCommandFactory) CreateCommand(name string, args ...string) Command {
	return &ExecCommand{cmd: exec.Command(name, args...)}
}

// ExecCommand wraps exec.Cmd to implement interfaces.Command
type ExecCommand struct {
	cmd *exec.Cmd
}

func (e *ExecCommand) Start() error {
	return e.cmd.Start()
}

func (e *ExecCommand) Wait() error {
	return e.cmd.Wait()
}

func (e *ExecCommand) Process() Process {
	if e.cmd.Process == nil {
		return nil
	}
	return &ExecProcess{process: e.cmd.Process}
}

func (e *ExecCommand) SetStdout(w interface{}) {
	e.cmd.Stdout = w.(io.Writer)
}

func (e *ExecCommand) SetStderr(w interface{}) {
	e.cmd.Stderr = w.(io.Writer)
}

func (e *ExecCommand) SetSysProcAttr(attr *syscall.SysProcAttr) {
	e.cmd.SysProcAttr = attr
}

func (e *ExecCommand) SetEnv(env []string) {
	e.cmd.Env = env
}

// ExecProcess wraps os.Process to implement interfaces.Process
type ExecProcess struct {
	process *os.Process
}

func (p *ExecProcess) Pid() int {
	return p.process.Pid
}

func (p *ExecProcess) Kill() error {
	return p.process.Kill()
}

// DefaultSyscall implements interfaces.SyscallInterface for macOS (mock/stub)
type DefaultSyscall struct{}

func (s *DefaultSyscall) Kill(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

func (s *DefaultSyscall) CreateProcessGroup() *syscall.SysProcAttr {
	// Return basic process group without Linux namespaces
	return &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
		// No Cloneflags - those are Linux-specific
	}
}

func (s *DefaultSyscall) Exec(argv0 string, argv []string, envv []string) error {
	return syscall.Exec(argv0, argv, envv)
}

func (s *DefaultSyscall) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	// Mock mount for macOS - always succeed for testing
	return nil
}

func (s *DefaultSyscall) Unmount(target string, flags int) error {
	// Mock unmount for macOS - always succeed for testing
	return nil
}

// DefaultOs implements interfaces.OsInterface using real os
type DefaultOs struct{}

func (d *DefaultOs) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

func (d *DefaultOs) MkdirAll(dir string, perm os.FileMode) error {
	return os.MkdirAll(dir, perm)
}

func (d *DefaultOs) Symlink(source string, path string) error {
	return os.Symlink(source, path)
}

func (d *DefaultOs) Remove(script string) error {
	return os.Remove(script)
}

func (d *DefaultOs) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (d *DefaultOs) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (d *DefaultOs) Executable() (string, error) {
	return os.Executable()
}

func (d *DefaultOs) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (d *DefaultOs) Environ() []string {
	return os.Environ()
}

func (d *DefaultOs) Getenv(key string) string {
	return os.Getenv(key)
}

func (d *DefaultOs) Getpid() int {
	return os.Getpid()
}

func (d *DefaultOs) Exit(code int) {
	os.Exit(code)
}

func (d *DefaultOs) Getuid() int {
	return os.Getuid()
}

func (d *DefaultOs) Getgid() int {
	return os.Getgid()
}

// DefaultExec implements interfaces.ExecInterface for jobinit
type DefaultExec struct{}

func (e *DefaultExec) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// ensure our types implement the interfaces
var _ OsInterface = (*DefaultOs)(nil)
var _ SyscallInterface = (*DefaultSyscall)(nil)
var _ ExecInterface = (*DefaultExec)(nil)
