package platform

// NewPlatform creates a Linux platform implementation
// Joblet is Linux-only, so no OS detection needed
func NewPlatform() Platform {
	return &LinuxPlatform{
		BasePlatform: NewBasePlatform(),
	}
}

// NewLinuxPlatform creates a Linux platform implementation
func NewLinuxPlatform() Platform {
	return NewPlatform()
}
