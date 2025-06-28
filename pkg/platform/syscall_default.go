//go:build linux

package platform

import "syscall"

// Linux-specific syscall constants
const (
	// Mount flags
	MountPrivate   = syscall.MS_PRIVATE
	MountRecursive = syscall.MS_REC
	MountBind      = syscall.MS_BIND
	MountReadOnly  = syscall.MS_RDONLY
	MountNoExec    = syscall.MS_NOEXEC
	MountNoSuid    = syscall.MS_NOSUID
	MountNoDev     = syscall.MS_NODEV

	// Unmount flags
	UnmountDetach = syscall.MNT_DETACH
	UnmountForce  = syscall.MNT_FORCE
	UnmountExpire = syscall.MNT_EXPIRE

	// Process signals
	SignalTerm = syscall.SIGTERM
	SignalKill = syscall.SIGKILL
	SignalStop = syscall.SIGSTOP
	SignalCont = syscall.SIGCONT

	// Clone flags for namespaces
	CloneNewPID    = syscall.CLONE_NEWPID
	CloneNewNS     = syscall.CLONE_NEWNS
	CloneNewNet    = syscall.CLONE_NEWNET
	CloneNewIPC    = syscall.CLONE_NEWIPC
	CloneNewUTS    = syscall.CLONE_NEWUTS
	CloneNewUser   = syscall.CLONE_NEWUSER
	CloneNewCgroup = syscall.CLONE_NEWCGROUP
)
