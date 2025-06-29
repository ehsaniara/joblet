//go:build !darwin

package platform

import "syscall"

// Default implementations for DarwinPlatform when NOT compiled on Darwin
// These provide fallback behavior when DarwinPlatform is used on non-Darwin systems

func (dp *DarwinPlatform) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	dp.logger.Warn("attempting Darwin mount operation on non-Darwin platform",
		"currentOS", "non-darwin", "source", source, "target", target)
	return DefaultMount("darwin", source, target, fstype, flags, data)
}

func (dp *DarwinPlatform) Unmount(target string, flags int) error {
	dp.logger.Warn("attempting Darwin unmount operation on non-Darwin platform",
		"currentOS", "non-darwin", "target", target)
	return DefaultUnmount("darwin", target, flags)
}

func (dp *DarwinPlatform) GetInfo() *Info {
	// Return Darwin platform info even when not on Darwin
	// This is useful for cross-platform information queries
	return &Info{
		OS:           "darwin",
		Architecture: "unknown", // Can't determine target arch
	}
}

func (dp *DarwinPlatform) ValidateRequirements() error {
	dp.logger.Error("cannot validate Darwin requirements on non-Darwin platform")
	return DefaultValidateRequirements("darwin")
}

func (dp *DarwinPlatform) CreateProcessGroup() *syscall.SysProcAttr {
	// Return basic process group (same as default)
	dp.logger.Debug("creating basic process group (Darwin-style)")
	return DefaultCreateProcessGroup()
}
