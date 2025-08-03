package server

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "joblet/api/gen"
	"joblet/internal/joblet/monitoring"
	"joblet/internal/joblet/monitoring/domain"
	"joblet/pkg/logger"
)

// MonitoringServiceServer implements the gRPC monitoring service
type MonitoringServiceServer struct {
	pb.UnimplementedMonitoringServiceServer
	monitor *monitoring.Service
	logger  *logger.Logger
}

// NewMonitoringServiceServer creates a new monitoring service server
func NewMonitoringServiceServer(monitor *monitoring.Service) *MonitoringServiceServer {
	return &MonitoringServiceServer{
		monitor: monitor,
		logger:  logger.WithField("component", "monitoring-grpc"),
	}
}

// GetSystemStatus returns the current system status
func (s *MonitoringServiceServer) GetSystemStatus(ctx context.Context, req *pb.EmptyRequest) (*pb.SystemStatusRes, error) {
	s.logger.Debug("GetSystemStatus called")

	systemStatus := s.monitor.GetSystemStatus()
	if systemStatus == nil {
		return nil, status.Errorf(codes.Internal, "failed to get system status")
	}

	return s.systemStatusToProto(systemStatus), nil
}

// StreamSystemMetrics streams system metrics at the specified interval
func (s *MonitoringServiceServer) StreamSystemMetrics(req *pb.StreamMetricsReq, stream pb.MonitoringService_StreamSystemMetricsServer) error {
	s.logger.Debug("StreamSystemMetrics called", "interval", req.IntervalSeconds, "filters", req.MetricTypes)

	// Default to 5 seconds if not specified
	interval := time.Duration(req.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}

	// Minimum interval of 1 second
	if interval < time.Second {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Send initial metrics immediately
	if err := s.sendMetrics(stream, req.MetricTypes); err != nil {
		return err
	}

	// Stream updates
	for {
		select {
		case <-stream.Context().Done():
			s.logger.Debug("stream cancelled by client")
			return nil
		case <-ticker.C:
			if err := s.sendMetrics(stream, req.MetricTypes); err != nil {
				return err
			}
		}
	}
}

// Helper methods

func (s *MonitoringServiceServer) sendMetrics(stream pb.MonitoringService_StreamSystemMetricsServer, filters []string) error {
	metrics := s.monitor.GetLatestMetrics()
	if metrics == nil {
		// No metrics available yet, skip this iteration
		return nil
	}

	filtered := s.filterMetrics(metrics, filters)
	proto := s.systemMetricsToProto(filtered)

	if err := stream.Send(proto); err != nil {
		s.logger.Error("failed to send metrics", "error", err)
		return status.Errorf(codes.Internal, "failed to send metrics: %v", err)
	}

	return nil
}

func (s *MonitoringServiceServer) filterMetrics(metrics *domain.SystemMetrics, types []string) *domain.SystemMetrics {
	if len(types) == 0 {
		return metrics // No filtering
	}

	// Create a filtered copy
	filtered := &domain.SystemMetrics{
		Timestamp: metrics.Timestamp,
		Host:      metrics.Host,
		Cloud:     metrics.Cloud,
	}

	// Apply filters
	for _, t := range types {
		switch t {
		case "cpu":
			filtered.CPU = metrics.CPU
		case "memory":
			filtered.Memory = metrics.Memory
		case "disk":
			filtered.Disk = metrics.Disk
		case "network":
			filtered.Network = metrics.Network
		case "io":
			filtered.IO = metrics.IO
		case "process":
			filtered.Processes = metrics.Processes
		}
	}

	return filtered
}

// Conversion methods from domain to protobuf

func (s *MonitoringServiceServer) systemStatusToProto(status *monitoring.SystemStatus) *pb.SystemStatusRes {
	return &pb.SystemStatusRes{
		Timestamp: status.Timestamp.Format(time.RFC3339),
		Available: status.Available,
		Host:      s.hostInfoToProto(status.Host),
		Cpu:       s.cpuMetricsToProto(status.CPU),
		Memory:    s.memoryMetricsToProto(status.Memory),
		Disks:     s.diskMetricsToProto(status.Disk),
		Networks:  s.networkMetricsToProto(status.Network),
		Io:        s.ioMetricsToProto(status.IO),
		Processes: s.processMetricsToProto(status.Processes),
		Cloud:     s.cloudInfoToProto(status.Cloud),
	}
}

func (s *MonitoringServiceServer) systemMetricsToProto(metrics *domain.SystemMetrics) *pb.SystemMetricsRes {
	return &pb.SystemMetricsRes{
		Timestamp: metrics.Timestamp.Format(time.RFC3339),
		Host:      s.hostInfoToProto(metrics.Host),
		Cpu:       s.cpuMetricsToProto(metrics.CPU),
		Memory:    s.memoryMetricsToProto(metrics.Memory),
		Disks:     s.diskMetricsToProto(metrics.Disk),
		Networks:  s.networkMetricsToProto(metrics.Network),
		Io:        s.ioMetricsToProto(metrics.IO),
		Processes: s.processMetricsToProto(metrics.Processes),
		Cloud:     s.cloudInfoToProto(metrics.Cloud),
	}
}

func (s *MonitoringServiceServer) hostInfoToProto(h domain.HostInfo) *pb.HostInfo {
	return &pb.HostInfo{
		Hostname:        h.Hostname,
		Os:              h.OS,
		Platform:        "", // Not available in domain model
		PlatformFamily:  "", // Not available in domain model
		PlatformVersion: "", // Not available in domain model
		KernelVersion:   h.Kernel,
		KernelArch:      "", // Not available in domain model
		Architecture:    h.Architecture,
		CpuCount:        0, // Not available in domain model
		TotalMemory:     0, // Not available in domain model
		BootTime:        h.BootTime.Format(time.RFC3339),
		Uptime:          int64(h.Uptime.Seconds()),
	}
}

func (s *MonitoringServiceServer) cpuMetricsToProto(c domain.CPUMetrics) *pb.CPUMetrics {
	loadAvg := make([]float64, len(c.LoadAverage))
	copy(loadAvg, c.LoadAverage[:])

	return &pb.CPUMetrics{
		Cores:        int32(c.Cores),
		UsagePercent: c.UsagePercent,
		UserTime:     c.UserTime,
		SystemTime:   c.SystemTime,
		IdleTime:     c.IdleTime,
		IoWaitTime:   c.IOWaitTime,
		StealTime:    c.StealTime,
		LoadAverage:  loadAvg,
		PerCoreUsage: c.PerCoreUsage,
	}
}

func (s *MonitoringServiceServer) memoryMetricsToProto(m domain.MemoryMetrics) *pb.MemoryMetrics {
	return &pb.MemoryMetrics{
		TotalBytes:     int64(m.TotalBytes),
		UsedBytes:      int64(m.UsedBytes),
		FreeBytes:      int64(m.FreeBytes),
		AvailableBytes: int64(m.AvailableBytes),
		UsagePercent:   m.UsagePercent,
		CachedBytes:    int64(m.CachedBytes),
		BufferedBytes:  int64(m.BufferedBytes),
		SwapTotal:      int64(m.SwapTotal),
		SwapUsed:       int64(m.SwapUsed),
		SwapFree:       int64(m.SwapFree),
	}
}

func (s *MonitoringServiceServer) diskMetricsToProto(disks []domain.DiskMetrics) []*pb.DiskMetrics {
	result := make([]*pb.DiskMetrics, len(disks))
	for i, d := range disks {
		result[i] = &pb.DiskMetrics{
			Device:             d.Device,
			MountPoint:         d.MountPoint,
			Filesystem:         d.FileSystem,
			TotalBytes:         int64(d.TotalBytes),
			UsedBytes:          int64(d.UsedBytes),
			FreeBytes:          int64(d.FreeBytes),
			UsagePercent:       d.UsagePercent,
			InodesTotal:        int64(d.InodesTotal),
			InodesUsed:         int64(d.InodesUsed),
			InodesFree:         int64(d.InodesFree),
			InodesUsagePercent: 0, // Not available in domain model
		}
	}
	return result
}

func (s *MonitoringServiceServer) networkMetricsToProto(networks []domain.NetworkMetrics) []*pb.NetworkMetrics {
	result := make([]*pb.NetworkMetrics, len(networks))
	for i, n := range networks {
		result[i] = &pb.NetworkMetrics{
			Interface:       n.Interface,
			BytesReceived:   int64(n.BytesReceived),
			BytesSent:       int64(n.BytesSent),
			PacketsReceived: int64(n.PacketsReceived),
			PacketsSent:     int64(n.PacketsSent),
			ErrorsIn:        int64(n.ErrorsReceived),
			ErrorsOut:       int64(n.ErrorsSent),
			DropsIn:         int64(n.DropsReceived),
			DropsOut:        int64(n.DropsSent),
			ReceiveRate:     n.RxThroughputBPS,
			TransmitRate:    n.TxThroughputBPS,
		}
	}
	return result
}

func (s *MonitoringServiceServer) ioMetricsToProto(io domain.IOMetrics) *pb.IOMetrics {
	// Domain IOMetrics doesn't have per-device breakdown like DiskIO
	// Return empty DiskIO array for now
	diskIO := make([]*pb.DiskIOMetrics, 0)

	return &pb.IOMetrics{
		TotalReads:  int64(io.ReadsCompleted),
		TotalWrites: int64(io.WritesCompleted),
		ReadBytes:   int64(io.ReadBytes),
		WriteBytes:  int64(io.WriteBytes),
		ReadRate:    0, // Not available in domain model
		WriteRate:   0, // Not available in domain model
		DiskIO:      diskIO,
	}
}

func (s *MonitoringServiceServer) processMetricsToProto(p domain.ProcessMetrics) *pb.ProcessMetrics {
	topCPU := make([]*pb.ProcessInfo, len(p.TopByCPU))
	for i, proc := range p.TopByCPU {
		topCPU[i] = s.processInfoToProto(proc)
	}

	topMem := make([]*pb.ProcessInfo, len(p.TopByMemory))
	for i, proc := range p.TopByMemory {
		topMem[i] = s.processInfoToProto(proc)
	}

	return &pb.ProcessMetrics{
		TotalProcesses:    int32(p.TotalProcesses),
		RunningProcesses:  int32(p.RunningProcesses),
		SleepingProcesses: 0, // Not available in domain model
		StoppedProcesses:  0, // Not available in domain model
		ZombieProcesses:   int32(p.ZombieProcesses),
		TotalThreads:      int32(p.TotalThreads),
		TopByCPU:          topCPU,
		TopByMemory:       topMem,
	}
}

func (s *MonitoringServiceServer) processInfoToProto(p domain.ProcessInfo) *pb.ProcessInfo {
	return &pb.ProcessInfo{
		Pid:           int32(p.PID),
		Ppid:          int32(p.PPID),
		Name:          p.Name,
		Command:       p.Command,
		CpuPercent:    p.CPUPercent,
		MemoryPercent: p.MemoryPercent,
		MemoryBytes:   int64(p.MemoryBytes),
		Status:        p.Status,
		StartTime:     p.StartTime.Format(time.RFC3339),
		User:          "", // Not available in domain model
	}
}

func (s *MonitoringServiceServer) cloudInfoToProto(c *domain.CloudInfo) *pb.CloudInfo {
	if c == nil {
		return nil
	}

	return &pb.CloudInfo{
		Provider:       c.Provider,
		Region:         c.Region,
		Zone:           c.Zone,
		InstanceID:     c.InstanceID,
		InstanceType:   c.InstanceType,
		HypervisorType: c.HypervisorType,
		Metadata:       c.Metadata,
	}
}
