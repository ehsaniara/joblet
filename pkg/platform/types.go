package platform

import "os"

// LinuxPlatform provides Linux-specific implementations
type LinuxPlatform struct {
	*BasePlatform
}

// DarwinPlatform provides macOS-specific implementations
type DarwinPlatform struct {
	*BasePlatform
}

func (dp *DarwinPlatform) RemoveAll(dir string) error {
	panic("implement me")
}

func (dp *DarwinPlatform) ReadDir(s string) ([]os.DirEntry, error) {
	panic("implement me")
}

// Ensure both platforms implement Platform interface
var _ Platform = (*LinuxPlatform)(nil)
var _ Platform = (*DarwinPlatform)(nil)
