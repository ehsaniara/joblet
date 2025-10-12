package server

import (
	"math"
	"sort"

	pb "github.com/ehsaniara/joblet/api/gen"
	"github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
)

// aggregateCPUMetrics calculates aggregate statistics for CPU usage
func (s *JobServiceServer) aggregateCPUMetrics(samples []*domain.JobMetricsSample) *pb.JobMetricsAggregate {
	if len(samples) == 0 {
		return nil
	}

	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, sample.CPU.UsagePercent)
	}

	return calculateAggregate(values)
}

// aggregateMemoryMetrics calculates aggregate statistics for memory usage
func (s *JobServiceServer) aggregateMemoryMetrics(samples []*domain.JobMetricsSample) *pb.JobMetricsAggregate {
	if len(samples) == 0 {
		return nil
	}

	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, sample.Memory.UsagePercent)
	}

	return calculateAggregate(values)
}

// aggregateIOMetrics calculates aggregate statistics for I/O throughput (combined read+write BPS)
func (s *JobServiceServer) aggregateIOMetrics(samples []*domain.JobMetricsSample) *pb.JobMetricsAggregate {
	if len(samples) == 0 {
		return nil
	}

	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		// Aggregate both read and write bandwidth
		totalBPS := sample.IO.ReadBPS + sample.IO.WriteBPS
		values = append(values, totalBPS)
	}

	return calculateAggregate(values)
}

// aggregateNetworkMetrics calculates aggregate statistics for network throughput (combined RX+TX BPS)
func (s *JobServiceServer) aggregateNetworkMetrics(samples []*domain.JobMetricsSample) *pb.JobMetricsAggregate {
	if len(samples) == 0 {
		return nil
	}

	// Count samples with network metrics
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		if sample.Network != nil {
			// Aggregate both receive and transmit bandwidth
			totalBPS := sample.Network.RxBPS + sample.Network.TxBPS
			values = append(values, totalBPS)
		}
	}

	if len(values) == 0 {
		return nil
	}

	return calculateAggregate(values)
}

// calculateAggregate computes min, max, avg, p50, p95, p99 for a set of values
func calculateAggregate(values []float64) *pb.JobMetricsAggregate {
	if len(values) == 0 {
		return &pb.JobMetricsAggregate{}
	}

	// Sort values for percentile calculation
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// Calculate min and max
	min := sorted[0]
	max := sorted[len(sorted)-1]

	// Calculate average
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	avg := sum / float64(len(values))

	// Calculate percentiles
	p50 := percentile(sorted, 0.50)
	p95 := percentile(sorted, 0.95)
	p99 := percentile(sorted, 0.99)

	return &pb.JobMetricsAggregate{
		Min: min,
		Max: max,
		Avg: avg,
		P50: p50,
		P95: p95,
		P99: p99,
	}
}

// percentile calculates the percentile value from a sorted slice
// p should be between 0 and 1 (e.g., 0.95 for 95th percentile)
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	// Calculate index using linear interpolation
	index := p * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sorted[lower]
	}

	// Linear interpolation between two values
	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}
