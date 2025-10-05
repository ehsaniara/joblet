package config

import (
	"testing"
	"time"
)

func TestJobMetricsConfig_Defaults(t *testing.T) {
	config := DefaultConfig.JobMetrics

	if config.Enabled != true {
		t.Error("Metrics should be enabled by default")
	}

	if config.DefaultSampleRate != 5*time.Second {
		t.Errorf("Expected default sample rate 5s, got %v", config.DefaultSampleRate)
	}

	if config.StorageDir != "/opt/joblet/metrics" {
		t.Errorf("Expected storage dir /opt/joblet/metrics, got %s", config.StorageDir)
	}

	if config.RetentionDays != 7 {
		t.Errorf("Expected retention 7 days, got %d", config.RetentionDays)
	}
}

func TestJobMetricsConfig_ToMetricsConfig(t *testing.T) {
	jmc := &JobMetricsConfig{
		Enabled:           true,
		DefaultSampleRate: 10 * time.Second,
		StorageDir:        "/custom/path",
		RetentionDays:     30,
	}

	mc := jmc.ToMetricsConfig()

	if mc == nil {
		t.Fatal("ToMetricsConfig returned nil")
	}

	if mc.Enabled != jmc.Enabled {
		t.Error("Enabled not converted correctly")
	}

	if mc.DefaultSampleRate != jmc.DefaultSampleRate {
		t.Error("DefaultSampleRate not converted correctly")
	}

	if mc.Storage.Directory != jmc.StorageDir {
		t.Error("StorageDir not converted correctly")
	}

	if mc.Storage.Retention.Days != jmc.RetentionDays {
		t.Error("RetentionDays not converted correctly")
	}
}

func TestJobMetricsConfig_DisabledByDefault(t *testing.T) {
	// Verify metrics are enabled as per requirements
	if !DefaultConfig.JobMetrics.Enabled {
		t.Error("Metrics should be enabled by default as per requirements")
	}
}
