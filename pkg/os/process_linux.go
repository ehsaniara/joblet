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

// SetEnv sets the environment variables for the command
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

// DefaultSyscall implements interfaces.SyscallInterface using real syscalls
type DefaultSyscall struct{}

func (s *DefaultSyscall) PivotRoot(newRoot, oldRoot string) error {
	return syscall.PivotRoot(newRoot, oldRoot)
}

func (s *DefaultSyscall) Chroot(path string) error {
	return syscall.Chroot(path)
}

func (s *DefaultSyscall) Unmount(target string, flags int) error {
	return syscall.Unmount(target, flags)
}

func (s *DefaultSyscall) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return syscall.Mount(source, target, fstype, flags, data)
}

func (s *DefaultSyscall) Exec(argv0 string, argv []string, envv []string) error {
	return syscall.Exec(argv0, argv, envv)
}

func (s *DefaultSyscall) Kill(pid int, sig syscall.Signal) error {
	// Direct passthrough to syscall.Kill following Unix convention:
	// - Positive pid: kills the specific process
	// - Negative pid: kills the process group (IMPORTANT: pid must be negative for group)
	return syscall.Kill(pid, sig)
}

func (s *DefaultSyscall) CreateProcessGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
		Pgid:    0,    // Use process PID as group ID
	}
}

// DefaultOs implements interfaces.OsInterface using real os
type DefaultOs struct{}

func (d *DefaultOs) ReadDir(dirname string) ([]os.DirEntry, error) {
	return os.ReadDir(dirname)
}

func (d *DefaultOs) Setenv(key, value string) error {
	return os.Setenv(key, value)
}

func (d *DefaultOs) RemoveAll(root string) error {
	return os.RemoveAll(root)
}

func (d *DefaultOs) Chdir(dir string) error {
	return os.Chdir(dir)
}

func (d *DefaultOs) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

func (d *DefaultOs) Getuid() int {
	return os.Getuid()
}

func (d *DefaultOs) Getgid() int {
	return os.Getgid()
}

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

// DefaultExec implements interfaces.ExecInterface for jobinit
type DefaultExec struct{}

func (e *DefaultExec) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// ensure our types implement the interfaces
var _ OsInterface = (*DefaultOs)(nil)
var _ SyscallInterface = (*DefaultSyscall)(nil)
var _ ExecInterface = (*DefaultExec)(nil)
