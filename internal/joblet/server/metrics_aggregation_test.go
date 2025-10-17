package server

import (
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
)

func TestCalculateAggregate(t *testing.T) {
	tests := []struct {
		name    string
		values  []float64
		wantMin float64
		wantMax float64
		wantAvg float64
		wantP50 float64
		wantP95 float64
		wantP99 float64
	}{
		{
			name:    "empty slice",
			values:  []float64{},
			wantMin: 0,
			wantMax: 0,
			wantAvg: 0,
			wantP50: 0,
			wantP95: 0,
			wantP99: 0,
		},
		{
			name:    "single value",
			values:  []float64{50.0},
			wantMin: 50.0,
			wantMax: 50.0,
			wantAvg: 50.0,
			wantP50: 50.0,
			wantP95: 50.0,
			wantP99: 50.0,
		},
		{
			name:    "multiple values",
			values:  []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
			wantMin: 10.0,
			wantMax: 100.0,
			wantAvg: 55.0,
			wantP50: 55.0, // median of 10 values
			wantP95: 95.5, // 95th percentile
			wantP99: 99.1, // 99th percentile
		},
		{
			name:    "unsorted values",
			values:  []float64{90, 10, 50, 30, 70, 20, 80, 40, 60, 100},
			wantMin: 10.0,
			wantMax: 100.0,
			wantAvg: 55.0,
			wantP50: 55.0,
			wantP95: 95.5,
			wantP99: 99.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateAggregate(tt.values)

			if result.Min != tt.wantMin {
				t.Errorf("Min = %v, want %v", result.Min, tt.wantMin)
			}
			if result.Max != tt.wantMax {
				t.Errorf("Max = %v, want %v", result.Max, tt.wantMax)
			}
			if result.Avg != tt.wantAvg {
				t.Errorf("Avg = %v, want %v", result.Avg, tt.wantAvg)
			}
			if result.P50 != tt.wantP50 {
				t.Errorf("P50 = %v, want %v", result.P50, tt.wantP50)
			}
			if result.P95 != tt.wantP95 {
				t.Errorf("P95 = %v, want %v", result.P95, tt.wantP95)
			}
			if result.P99 != tt.wantP99 {
				t.Errorf("P99 = %v, want %v", result.P99, tt.wantP99)
			}
		})
	}
}

func TestAggregateCPUMetrics(t *testing.T) {
	server := &WorkflowServiceServer{}

	samples := []*domain.JobMetricsSample{
		{
			JobID:     "test-job",
			Timestamp: time.Now(),
			CPU: domain.CPUMetrics{
				UsagePercent: 25.0,
			},
		},
		{
			JobID:     "test-job",
			Timestamp: time.Now(),
			CPU: domain.CPUMetrics{
				UsagePercent: 50.0,
			},
		},
		{
			JobID:     "test-job",
			Timestamp: time.Now(),
			CPU: domain.CPUMetrics{
				UsagePercent: 75.0,
			},
		},
	}

	result := server.aggregateCPUMetrics(samples)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Min != 25.0 {
		t.Errorf("Min = %v, want 25.0", result.Min)
	}
	if result.Max != 75.0 {
		t.Errorf("Max = %v, want 75.0", result.Max)
	}
	if result.Avg != 50.0 {
		t.Errorf("Avg = %v, want 50.0", result.Avg)
	}
	if result.P50 != 50.0 {
		t.Errorf("P50 = %v, want 50.0", result.P50)
	}
}

func TestAggregateMemoryMetrics(t *testing.T) {
	server := &WorkflowServiceServer{}

	samples := []*domain.JobMetricsSample{
		{
			JobID:     "test-job",
			Timestamp: time.Now(),
			Memory: domain.MemoryMetrics{
				UsagePercent: 30.0,
			},
		},
		{
			JobID:     "test-job",
			Timestamp: time.Now(),
			Memory: domain.MemoryMetrics{
				UsagePercent: 60.0,
			},
		},
		{
			JobID:     "test-job",
			Timestamp: time.Now(),
			Memory: domain.MemoryMetrics{
				UsagePercent: 90.0,
			},
		},
	}

	result := server.aggregateMemoryMetrics(samples)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Min != 30.0 {
		t.Errorf("Min = %v, want 30.0", result.Min)
	}
	if result.Max != 90.0 {
		t.Errorf("Max = %v, want 90.0", result.Max)
	}
	if result.Avg != 60.0 {
		t.Errorf("Avg = %v, want 60.0", result.Avg)
	}
}

func TestAggregateIOMetrics(t *testing.T) {
	server := &WorkflowServiceServer{}

	samples := []*domain.JobMetricsSample{
		{
			JobID:     "test-job",
			Timestamp: time.Now(),
			IO: domain.IOMetrics{
				ReadBPS:  1000.0,
				WriteBPS: 500.0,
			},
		},
		{
			JobID:     "test-job",
			Timestamp: time.Now(),
			IO: domain.IOMetrics{
				ReadBPS:  2000.0,
				WriteBPS: 1000.0,
			},
		},
	}

	result := server.aggregateIOMetrics(samples)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Total BPS should be read + write
	if result.Min != 1500.0 {
		t.Errorf("Min = %v, want 1500.0", result.Min)
	}
	if result.Max != 3000.0 {
		t.Errorf("Max = %v, want 3000.0", result.Max)
	}
}

func TestAggregateNetworkMetrics(t *testing.T) {
	server := &WorkflowServiceServer{}

	samples := []*domain.JobMetricsSample{
		{
			JobID:     "test-job",
			Timestamp: time.Now(),
			Network: &domain.NetworkMetrics{
				RxBPS: 1000.0,
				TxBPS: 500.0,
			},
		},
		{
			JobID:     "test-job",
			Timestamp: time.Now(),
			Network: &domain.NetworkMetrics{
				RxBPS: 2000.0,
				TxBPS: 1000.0,
			},
		},
		{
			JobID:     "test-job",
			Timestamp: time.Now(),
			Network:   nil, // Sample without network metrics
		},
	}

	result := server.aggregateNetworkMetrics(samples)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should only include samples with network metrics
	// Total BPS should be RX + TX
	if result.Min != 1500.0 {
		t.Errorf("Min = %v, want 1500.0", result.Min)
	}
	if result.Max != 3000.0 {
		t.Errorf("Max = %v, want 3000.0", result.Max)
	}
}

func TestAggregateMetrics_EmptySamples(t *testing.T) {
	server := &WorkflowServiceServer{}

	result := server.aggregateCPUMetrics([]*domain.JobMetricsSample{})
	if result != nil {
		t.Errorf("Expected nil result for empty samples, got %v", result)
	}

	result = server.aggregateMemoryMetrics([]*domain.JobMetricsSample{})
	if result != nil {
		t.Errorf("Expected nil result for empty samples, got %v", result)
	}

	result = server.aggregateIOMetrics([]*domain.JobMetricsSample{})
	if result != nil {
		t.Errorf("Expected nil result for empty samples, got %v", result)
	}

	result = server.aggregateNetworkMetrics([]*domain.JobMetricsSample{})
	if result != nil {
		t.Errorf("Expected nil result for empty samples, got %v", result)
	}
}
