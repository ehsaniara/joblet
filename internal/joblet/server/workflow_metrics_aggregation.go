package server

import (
	pb "github.com/ehsaniara/joblet/api/gen"
	"github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
)

// aggregateCPUMetrics calculates aggregate statistics for CPU usage
func (s *WorkflowServiceServer) aggregateCPUMetrics(samples []*domain.JobMetricsSample) *pb.JobMetricsAggregate {
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
func (s *WorkflowServiceServer) aggregateMemoryMetrics(samples []*domain.JobMetricsSample) *pb.JobMetricsAggregate {
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
func (s *WorkflowServiceServer) aggregateIOMetrics(samples []*domain.JobMetricsSample) *pb.JobMetricsAggregate {
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
func (s *WorkflowServiceServer) aggregateNetworkMetrics(samples []*domain.JobMetricsSample) *pb.JobMetricsAggregate {
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
