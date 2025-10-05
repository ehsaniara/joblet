package config

import (
	"joblet/internal/joblet/metrics/domain"
)

// ToMetricsConfig converts the pkg/config JobMetricsConfig to the internal domain MetricsConfig
func (jmc *JobMetricsConfig) ToMetricsConfig() *domain.MetricsConfig {
	return &domain.MetricsConfig{
		Enabled:           jmc.Enabled,
		DefaultSampleRate: jmc.DefaultSampleRate,
		Storage: domain.StorageConfig{
			Directory: jmc.StorageDir,
			Retention: domain.RetentionConfig{
				Days: jmc.RetentionDays,
			},
		},
	}
}
