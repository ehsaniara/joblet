package platform

import (
	"io"
	"os"
	"os/exec"
	"syscall"
	"worker/pkg/logger"
)

// BasePlatform provides common functionality shared across platforms
type BasePlatform struct {
	logger *logger.Logger
}

// NewBasePlatform creates a new base platform
func NewBasePlatform() *BasePlatform {
	return &BasePlatform{
		logger: logger.New().WithField("component", "platform"),
	}
}

// Common OS operations that work the same across platforms
func (bp *BasePlatform) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (bp *BasePlatform) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (bp *BasePlatform) Remove(path string) error {
	return os.Remove(path)
}

func (bp *BasePlatform) Symlink(source string, path string) error {
	return os.Symlink(source, path)
}

func (bp *BasePlatform) MkdirAll(dir string, perm os.FileMode) error {
	return os.MkdirAll(dir, perm)
}

func (bp *BasePlatform) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (bp *BasePlatform) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

func (bp *BasePlatform) Executable() (string, error) {
	return os.Executable()
}

func (bp *BasePlatform) Getpid() int {
	return os.Getpid()
}

func (bp *BasePlatform) Exit(code int) {
	os.Exit(code)
}

func (bp *BasePlatform) Environ() []string {
	return os.Environ()
}

func (bp *BasePlatform) Getenv(key string) string {
	return os.Getenv(key)
}

func (bp *BasePlatform) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// Common syscall operations
func (bp *BasePlatform) Kill(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

func (bp *BasePlatform) Exec(argv0 string, argv []string, envv []string) error {
	return syscall.Exec(argv0, argv, envv)
}

func (bp *BasePlatform) CreateProcessGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
}

// Common command operations
func (bp *BasePlatform) CreateCommand(name string, args ...string) Command {
	return &ExecCommand{cmd: exec.Command(name, args...)}
}

func (bp *BasePlatform) IsExist(err error) bool {
	return os.IsExist(err)
}

func (lp *LinuxPlatform) RemoveAll(dir string) error {
	return os.RemoveAll(dir)
}

func (lp *LinuxPlatform) ReadDir(s string) ([]os.DirEntry, error) {
	return os.ReadDir(s)
}

// ExecCommand wraps exec.Cmd to implement Command interface
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

// ExecProcess wraps os.Process to implement Process interface
type ExecProcess struct {
	process *os.Process
}

func (p *ExecProcess) Pid() int {
	return p.process.Pid
}

func (p *ExecProcess) Kill() error {
	return p.process.Kill()
}
