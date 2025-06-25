package platform

import (
	"fmt"
	"runtime"
	"sync"
)

var (
	currentPlatform Platform
	platformOnce    sync.Once
)

// NewPlatform creates a platform-specific implementation
func NewPlatform() Platform {
	platformOnce.Do(func() {
		currentPlatform = createPlatform()
	})
	return currentPlatform
}

// createPlatform creates the appropriate platform implementation
func createPlatform() Platform {
	switch runtime.GOOS {
	case "linux":
		return &LinuxPlatform{
			BasePlatform: NewBasePlatform(),
		}
	case "darwin":
		return &DarwinPlatform{
			BasePlatform: NewBasePlatform(),
		}
	default:
		panic(fmt.Sprintf("unsupported platform: %s", runtime.GOOS))
	}
}

// GetCurrentPlatform returns the singleton platform instance
func GetCurrentPlatform() Platform {
	return NewPlatform()
}

// GetPlatformInfo returns information about current platform capabilities
func GetPlatformInfo() *PlatformInfo {
	return GetCurrentPlatform().GetInfo()
}
