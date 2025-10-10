package cloud

import "time"

// Cloud detection constants
const (
	// AWS EC2 IMDS (Instance Metadata Service) Configuration
	AWSIMDSEndpoint   = "http://169.254.169.254"
	AWSIMDSTokenTTL   = "21600" // 6 hours in seconds
	AWSIMDSTimeout    = 2 * time.Second
	AWSIMDSAPIVersion = "latest"

	// Azure IMDS Configuration
	AzureIMDSEndpoint   = "http://169.254.169.254"
	AzureIMDSAPIVersion = "2021-02-01"

	// Detection Timeouts
	DefaultDetectionTimeout = 5 * time.Second

	// Cache Duration
	DetectionCacheDuration = 1 * time.Hour
)

// Cloud Provider Names (actively used)
const (
	ProviderAWS   = "AWS"
	ProviderAzure = "Azure"
)
