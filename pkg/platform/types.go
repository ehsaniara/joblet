package platform

// LinuxPlatform provides Linux-specific implementations
type LinuxPlatform struct {
	*BasePlatform
}

// DarwinPlatform provides macOS-specific implementations
type DarwinPlatform struct {
	*BasePlatform
}

// Ensure both platforms implement Platform interface
var _ Platform = (*LinuxPlatform)(nil)
var _ Platform = (*DarwinPlatform)(nil)
