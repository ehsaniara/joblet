package server

import (
	pb "joblet/api/gen"
	"joblet/internal/joblet/metrics/domain"
)

// convertMetricsSampleToProto converts domain JobMetricsSample to protobuf
func convertMetricsSampleToProto(sample *domain.JobMetricsSample) *pb.JobMetricsSample {
	if sample == nil {
		return nil
	}

	pbSample := &pb.JobMetricsSample{
		JobId:                 sample.JobID,
		Timestamp:             sample.Timestamp.Unix(),
		SampleIntervalSeconds: int32(sample.SampleInterval.Seconds()),
		Cpu:                   convertCPUMetricsToProto(&sample.CPU),
		Memory:                convertMemoryMetricsToProto(&sample.Memory),
		Io:                    convertIOMetricsToProto(&sample.IO),
		Process:               convertProcessMetricsToProto(&sample.Process),
		CgroupPath:            sample.CgroupPath,
		GpuAllocation:         make([]int32, len(sample.GPUAllocation)),
	}

	// Convert GPU allocation
	for i, gpu := range sample.GPUAllocation {
		pbSample.GpuAllocation[i] = int32(gpu)
	}

	// Convert limits if present
	if sample.Limits != nil {
		pbSample.Limits = &pb.JobResourceLimits{
			Cpu:    sample.Limits.CPU,
			Memory: sample.Limits.Memory,
			Io:     sample.Limits.IO,
		}
	}

	// Convert network metrics if present
	if sample.Network != nil {
		pbSample.Network = convertNetworkMetricsToProto(sample.Network)
	}

	// Convert GPU metrics if present
	if len(sample.GPU) > 0 {
		pbSample.Gpu = make([]*pb.JobGPUMetrics, len(sample.GPU))
		for i, gpu := range sample.GPU {
			pbSample.Gpu[i] = convertGPUMetricsToProto(&gpu)
		}
	}

	return pbSample
}

func convertCPUMetricsToProto(cpu *domain.CPUMetrics) *pb.JobCPUMetrics {
	return &pb.JobCPUMetrics{
		UsageUsec:       cpu.UsageUSec,
		UserUsec:        cpu.UserUSec,
		SystemUsec:      cpu.SystemUSec,
		NrPeriods:       cpu.NrPeriods,
		NrThrottled:     cpu.NrThrottled,
		ThrottledUsec:   cpu.ThrottledUSec,
		UsagePercent:    cpu.UsagePercent,
		ThrottlePercent: cpu.ThrottlePercent,
		PressureSome10:  cpu.PressureSome10,
		PressureSome60:  cpu.PressureSome60,
		PressureSome300: cpu.PressureSome300,
		PressureFull10:  cpu.PressureFull10,
		PressureFull60:  cpu.PressureFull60,
		PressureFull300: cpu.PressureFull300,
	}
}

func convertMemoryMetricsToProto(mem *domain.MemoryMetrics) *pb.JobMemoryMetrics {
	return &pb.JobMemoryMetrics{
		Current:         mem.Current,
		Max:             mem.Max,
		UsagePercent:    mem.UsagePercent,
		Anon:            mem.Anon,
		File:            mem.File,
		KernelStack:     mem.KernelStack,
		Slab:            mem.Slab,
		Sock:            mem.Sock,
		Shmem:           mem.Shmem,
		FileMapped:      mem.FileMapped,
		FileDirty:       mem.FileDirty,
		FileWriteback:   mem.FileWriteback,
		PgFault:         mem.PgFault,
		PgMajFault:      mem.PgMajFault,
		OomEvents:       mem.OOMEvents,
		OomKill:         mem.OOMKill,
		PressureSome10:  mem.PressureSome10,
		PressureSome60:  mem.PressureSome60,
		PressureSome300: mem.PressureSome300,
		PressureFull10:  mem.PressureFull10,
		PressureFull60:  mem.PressureFull60,
		PressureFull300: mem.PressureFull300,
	}
}

func convertIOMetricsToProto(io *domain.IOMetrics) *pb.JobIOMetrics {
	pbIO := &pb.JobIOMetrics{
		Devices:           make(map[string]*pb.DeviceIOMetrics),
		TotalReadBytes:    io.TotalReadBytes,
		TotalWriteBytes:   io.TotalWriteBytes,
		TotalReadOps:      io.TotalReadOps,
		TotalWriteOps:     io.TotalWriteOps,
		TotalDiscardBytes: io.TotalDiscardBytes,
		TotalDiscardOps:   io.TotalDiscardOps,
		ReadBPS:           io.ReadBPS,
		WriteBPS:          io.WriteBPS,
		ReadIOPS:          io.ReadIOPS,
		WriteIOPS:         io.WriteIOPS,
		PressureSome10:    io.PressureSome10,
		PressureSome60:    io.PressureSome60,
		PressureSome300:   io.PressureSome300,
		PressureFull10:    io.PressureFull10,
		PressureFull60:    io.PressureFull60,
		PressureFull300:   io.PressureFull300,
	}

	// Convert per-device metrics
	for device, metrics := range io.Devices {
		pbIO.Devices[device] = &pb.DeviceIOMetrics{
			Device:       metrics.Device,
			ReadBytes:    metrics.ReadBytes,
			WriteBytes:   metrics.WriteBytes,
			ReadOps:      metrics.ReadOps,
			WriteOps:     metrics.WriteOps,
			DiscardBytes: metrics.DiscardBytes,
			DiscardOps:   metrics.DiscardOps,
		}
	}

	return pbIO
}

func convertNetworkMetricsToProto(net *domain.NetworkMetrics) *pb.JobNetworkMetrics {
	pbNet := &pb.JobNetworkMetrics{
		Interfaces:     make(map[string]*pb.NetworkInterfaceMetrics),
		TotalRxBytes:   net.TotalRxBytes,
		TotalTxBytes:   net.TotalTxBytes,
		TotalRxPackets: net.TotalRxPackets,
		TotalTxPackets: net.TotalTxPackets,
		TotalRxErrors:  net.TotalRxErrors,
		TotalTxErrors:  net.TotalTxErrors,
		TotalRxDropped: net.TotalRxDropped,
		TotalTxDropped: net.TotalTxDropped,
		RxBPS:          net.RxBPS,
		TxBPS:          net.TxBPS,
	}

	// Convert per-interface metrics
	for iface, metrics := range net.Interfaces {
		pbNet.Interfaces[iface] = &pb.NetworkInterfaceMetrics{
			Interface: metrics.Interface,
			RxBytes:   metrics.RxBytes,
			TxBytes:   metrics.TxBytes,
			RxPackets: metrics.RxPackets,
			TxPackets: metrics.TxPackets,
			RxErrors:  metrics.RxErrors,
			TxErrors:  metrics.TxErrors,
			RxDropped: metrics.RxDropped,
			TxDropped: metrics.TxDropped,
		}
	}

	return pbNet
}

func convertProcessMetricsToProto(proc *domain.ProcessMetrics) *pb.JobProcessMetrics {
	return &pb.JobProcessMetrics{
		Current:  proc.Current,
		Max:      proc.Max,
		Events:   proc.Events,
		Threads:  proc.Threads,
		Running:  proc.Running,
		Sleeping: proc.Sleeping,
		Stopped:  proc.Stopped,
		Zombie:   proc.Zombie,
		OpenFDs:  proc.OpenFDs,
		MaxFDs:   proc.MaxFDs,
	}
}

func convertGPUMetricsToProto(gpu *domain.GPUMetrics) *pb.JobGPUMetrics {
	return &pb.JobGPUMetrics{
		Index:             int32(gpu.Index),
		Uuid:              gpu.UUID,
		Name:              gpu.Name,
		ComputeCapability: gpu.ComputeCapability,
		DriverVersion:     gpu.DriverVersion,
		Utilization:       gpu.Utilization,
		MemoryUsed:        gpu.MemoryUsed,
		MemoryTotal:       gpu.MemoryTotal,
		MemoryFree:        gpu.MemoryFree,
		MemoryPercent:     gpu.MemoryPercent,
		EncoderUtil:       gpu.EncoderUtil,
		DecoderUtil:       gpu.DecoderUtil,
		SmClock:           uint64(gpu.SMClock),
		MemoryClock:       uint64(gpu.MemoryClock),
		PcieThroughputRx:  gpu.PCIeThroughputRx,
		PcieThroughputTx:  gpu.PCIeThroughputTx,
		Temperature:       gpu.Temperature,
		TemperatureMemory: gpu.TemperatureMemory,
		PowerDraw:         gpu.PowerDraw,
		PowerLimit:        gpu.PowerLimit,
		FanSpeed:          gpu.FanSpeed,
		EccErrorsSingle:   gpu.ECCErrorsSingle,
		EccErrorsDouble:   gpu.ECCErrorsDouble,
		XidErrors:         gpu.XIDErrors,
		RetiredPages:      gpu.RetiredPages,
		ThrottleReasons:   gpu.ThrottleReasons,
		ProcessesCount:    gpu.ProcessesCount,
		ProcessesMemory:   gpu.ProcessesMemory,
		ComputeMode:       gpu.ComputeMode,
	}
}
