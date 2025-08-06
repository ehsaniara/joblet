package platform

// LinuxPlatform provides Linux-specific implementations
type LinuxPlatform struct {
	*BasePlatform
}

// Ensure LinuxPlatform implements Platform interface
var _ Platform = (*LinuxPlatform)(nil)
