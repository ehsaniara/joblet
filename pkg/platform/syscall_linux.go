//go:build !linux

package platform

// Default syscall constants for non-Linux platforms
// These are dummy values that allow compilation on non-Linux systems
const (
	// Mount flags (dummy values)
	MountPrivate   = 0x40000
	MountRecursive = 0x4000
	MountBind      = 0x1000
	MountReadOnly  = 0x1
	MountNoExec    = 0x8
	MountNoSuid    = 0x2
	MountNoDev     = 0x4

	// Unmount flags (dummy values)
	UnmountDetach = 0x2
	UnmountForce  = 0x1
	UnmountExpire = 0x4

	// Process signals (use standard values where possible)
	SignalTerm = 15 // SIGTERM
	SignalKill = 9  // SIGKILL
	SignalStop = 19 // SIGSTOP
	SignalCont = 18 // SIGCONT

	// Clone flags (dummy values for non-Linux)
	CloneNewPID    = 0x20000000
	CloneNewNS     = 0x00020000
	CloneNewNet    = 0x40000000
	CloneNewIPC    = 0x08000000
	CloneNewUTS    = 0x04000000
	CloneNewUser   = 0x10000000
	CloneNewCgroup = 0x02000000
)
