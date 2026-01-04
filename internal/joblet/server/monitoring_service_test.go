package server

import (
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/monitoring/domain"
	"github.com/ehsaniara/joblet/pkg/config"
)

func newTestMonitoringServiceServer() *MonitoringServiceServer {
	cfg := &config.Config{
		Server: config.ServerConfig{
			NodeId: "test-node",
		},
	}
	return &MonitoringServiceServer{
		config: cfg,
	}
}

func TestDiskMetricsToProto_InodesUsagePercent(t *testing.T) {
	server := newTestMonitoringServiceServer()

	tests := []struct {
		name           string
		disks          []domain.DiskMetrics
		expectedInodes []float64
	}{
		{
			name: "normal inodes usage",
			disks: []domain.DiskMetrics{
				{
					Device:      "/dev/sda1",
					MountPoint:  "/",
					FileSystem:  "ext4",
					TotalBytes:  100000000000,
					UsedBytes:   50000000000,
					FreeBytes:   50000000000,
					InodesTotal: 1000000,
					InodesUsed:  250000,
					InodesFree:  750000,
				},
			},
			expectedInodes: []float64{25.0}, // 250000/1000000 * 100 = 25%
		},
		{
			name: "zero inodes total",
			disks: []domain.DiskMetrics{
				{
					Device:      "/dev/sda1",
					MountPoint:  "/",
					FileSystem:  "ext4",
					InodesTotal: 0, // No inodes info available
					InodesUsed:  0,
					InodesFree:  0,
				},
			},
			expectedInodes: []float64{0.0}, // Should be 0 when total is 0
		},
		{
			name: "full inodes usage",
			disks: []domain.DiskMetrics{
				{
					Device:      "/dev/sda1",
					MountPoint:  "/",
					FileSystem:  "ext4",
					InodesTotal: 500000,
					InodesUsed:  500000,
					InodesFree:  0,
				},
			},
			expectedInodes: []float64{100.0}, // 100% usage
		},
		{
			name: "multiple disks",
			disks: []domain.DiskMetrics{
				{
					Device:      "/dev/sda1",
					MountPoint:  "/",
					FileSystem:  "ext4",
					InodesTotal: 1000000,
					InodesUsed:  100000,
					InodesFree:  900000,
				},
				{
					Device:      "/dev/sdb1",
					MountPoint:  "/data",
					FileSystem:  "xfs",
					InodesTotal: 2000000,
					InodesUsed:  1000000,
					InodesFree:  1000000,
				},
			},
			expectedInodes: []float64{10.0, 50.0}, // 10% and 50%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.diskMetricsToProto(tt.disks)

			if len(result) != len(tt.disks) {
				t.Fatalf("Expected %d disks, got %d", len(tt.disks), len(result))
			}

			for i, expected := range tt.expectedInodes {
				actual := result[i].InodesUsagePercent
				if actual != expected {
					t.Errorf("Disk %d: expected InodesUsagePercent=%.2f%%, got %.2f%%",
						i, expected, actual)
				}
			}
		})
	}
}

func TestDiskMetricsToProto_AllFields(t *testing.T) {
	server := newTestMonitoringServiceServer()

	disk := domain.DiskMetrics{
		Device:       "/dev/nvme0n1p1",
		MountPoint:   "/home",
		FileSystem:   "ext4",
		TotalBytes:   500000000000,
		UsedBytes:    200000000000,
		FreeBytes:    300000000000,
		UsagePercent: 40.0,
		InodesTotal:  32000000,
		InodesUsed:   1600000,
		InodesFree:   30400000,
	}

	result := server.diskMetricsToProto([]domain.DiskMetrics{disk})

	if len(result) != 1 {
		t.Fatalf("Expected 1 disk, got %d", len(result))
	}

	proto := result[0]
	if proto.Device != disk.Device {
		t.Errorf("Device: expected %s, got %s", disk.Device, proto.Device)
	}
	if proto.MountPoint != disk.MountPoint {
		t.Errorf("MountPoint: expected %s, got %s", disk.MountPoint, proto.MountPoint)
	}
	if proto.Filesystem != disk.FileSystem {
		t.Errorf("Filesystem: expected %s, got %s", disk.FileSystem, proto.Filesystem)
	}
	if proto.TotalBytes != int64(disk.TotalBytes) {
		t.Errorf("TotalBytes: expected %d, got %d", disk.TotalBytes, proto.TotalBytes)
	}
	if proto.UsedBytes != int64(disk.UsedBytes) {
		t.Errorf("UsedBytes: expected %d, got %d", disk.UsedBytes, proto.UsedBytes)
	}
	if proto.FreeBytes != int64(disk.FreeBytes) {
		t.Errorf("FreeBytes: expected %d, got %d", disk.FreeBytes, proto.FreeBytes)
	}
	if proto.UsagePercent != disk.UsagePercent {
		t.Errorf("UsagePercent: expected %.2f, got %.2f", disk.UsagePercent, proto.UsagePercent)
	}
	if proto.InodesTotal != int64(disk.InodesTotal) {
		t.Errorf("InodesTotal: expected %d, got %d", disk.InodesTotal, proto.InodesTotal)
	}
	if proto.InodesUsed != int64(disk.InodesUsed) {
		t.Errorf("InodesUsed: expected %d, got %d", disk.InodesUsed, proto.InodesUsed)
	}
	if proto.InodesFree != int64(disk.InodesFree) {
		t.Errorf("InodesFree: expected %d, got %d", disk.InodesFree, proto.InodesFree)
	}

	// InodesUsagePercent: 1600000/32000000 * 100 = 5%
	expectedInodesUsage := 5.0
	if proto.InodesUsagePercent != expectedInodesUsage {
		t.Errorf("InodesUsagePercent: expected %.2f%%, got %.2f%%",
			expectedInodesUsage, proto.InodesUsagePercent)
	}
}

func TestIOMetricsToProto_BasicFields(t *testing.T) {
	server := newTestMonitoringServiceServer()

	io := domain.IOMetrics{
		ReadsCompleted:  10000,
		WritesCompleted: 5000,
		ReadBytes:       102400000,
		WriteBytes:      51200000,
		ReadRate:        1024000.5, // ~1MB/s
		WriteRate:       512000.25, // ~500KB/s
	}

	result := server.ioMetricsToProto(io)

	if result.TotalReads != int64(io.ReadsCompleted) {
		t.Errorf("TotalReads: expected %d, got %d", io.ReadsCompleted, result.TotalReads)
	}
	if result.TotalWrites != int64(io.WritesCompleted) {
		t.Errorf("TotalWrites: expected %d, got %d", io.WritesCompleted, result.TotalWrites)
	}
	if result.ReadBytes != int64(io.ReadBytes) {
		t.Errorf("ReadBytes: expected %d, got %d", io.ReadBytes, result.ReadBytes)
	}
	if result.WriteBytes != int64(io.WriteBytes) {
		t.Errorf("WriteBytes: expected %d, got %d", io.WriteBytes, result.WriteBytes)
	}
	if result.ReadRate != io.ReadRate {
		t.Errorf("ReadRate: expected %f, got %f", io.ReadRate, result.ReadRate)
	}
	if result.WriteRate != io.WriteRate {
		t.Errorf("WriteRate: expected %f, got %f", io.WriteRate, result.WriteRate)
	}
}

func TestIOMetricsToProto_PerDeviceMetrics(t *testing.T) {
	server := newTestMonitoringServiceServer()

	io := domain.IOMetrics{
		ReadsCompleted:  15000,
		WritesCompleted: 7500,
		ReadBytes:       153600000,
		WriteBytes:      76800000,
		ReadRate:        2048000.0,
		WriteRate:       1024000.0,
		PerDevice: []domain.DeviceIOMetrics{
			{
				Device:          "sda",
				ReadsCompleted:  10000,
				WritesCompleted: 5000,
				ReadBytes:       102400000,
				WriteBytes:      51200000,
				ReadTime:        5000,
				WriteTime:       2500,
				IOTime:          7500,
				Utilization:     75.0,
			},
			{
				Device:          "sdb",
				ReadsCompleted:  5000,
				WritesCompleted: 2500,
				ReadBytes:       51200000,
				WriteBytes:      25600000,
				ReadTime:        2500,
				WriteTime:       1250,
				IOTime:          3750,
				Utilization:     37.5,
			},
		},
	}

	result := server.ioMetricsToProto(io)

	if len(result.DiskIo) != 2 {
		t.Fatalf("Expected 2 per-device metrics, got %d", len(result.DiskIo))
	}

	// Check first device (sda)
	sda := result.DiskIo[0]
	if sda.Device != "sda" {
		t.Errorf("Device 0: expected 'sda', got '%s'", sda.Device)
	}
	if sda.ReadsCompleted != 10000 {
		t.Errorf("sda ReadsCompleted: expected 10000, got %d", sda.ReadsCompleted)
	}
	if sda.WritesCompleted != 5000 {
		t.Errorf("sda WritesCompleted: expected 5000, got %d", sda.WritesCompleted)
	}
	if sda.ReadBytes != 102400000 {
		t.Errorf("sda ReadBytes: expected 102400000, got %d", sda.ReadBytes)
	}
	if sda.WriteBytes != 51200000 {
		t.Errorf("sda WriteBytes: expected 51200000, got %d", sda.WriteBytes)
	}
	if sda.ReadTime != 5000 {
		t.Errorf("sda ReadTime: expected 5000, got %d", sda.ReadTime)
	}
	if sda.WriteTime != 2500 {
		t.Errorf("sda WriteTime: expected 2500, got %d", sda.WriteTime)
	}
	if sda.IoTime != 7500 {
		t.Errorf("sda IoTime: expected 7500, got %d", sda.IoTime)
	}
	if sda.Utilization != 75.0 {
		t.Errorf("sda Utilization: expected 75.0, got %f", sda.Utilization)
	}

	// Check second device (sdb)
	sdb := result.DiskIo[1]
	if sdb.Device != "sdb" {
		t.Errorf("Device 1: expected 'sdb', got '%s'", sdb.Device)
	}
	if sdb.Utilization != 37.5 {
		t.Errorf("sdb Utilization: expected 37.5, got %f", sdb.Utilization)
	}
}

func TestIOMetricsToProto_EmptyPerDevice(t *testing.T) {
	server := newTestMonitoringServiceServer()

	io := domain.IOMetrics{
		ReadsCompleted:  1000,
		WritesCompleted: 500,
		ReadRate:        100000.0,
		WriteRate:       50000.0,
		PerDevice:       nil, // No per-device metrics
	}

	result := server.ioMetricsToProto(io)

	if len(result.DiskIo) != 0 {
		t.Errorf("Expected empty DiskIo, got %d items", len(result.DiskIo))
	}

	// Basic fields should still be correct
	if result.ReadRate != io.ReadRate {
		t.Errorf("ReadRate: expected %f, got %f", io.ReadRate, result.ReadRate)
	}
}

func TestIOMetricsToProto_ZeroRates(t *testing.T) {
	server := newTestMonitoringServiceServer()

	io := domain.IOMetrics{
		ReadsCompleted:  0,
		WritesCompleted: 0,
		ReadBytes:       0,
		WriteBytes:      0,
		ReadRate:        0.0,
		WriteRate:       0.0,
		PerDevice: []domain.DeviceIOMetrics{
			{
				Device:      "sda",
				Utilization: 0.0,
			},
		},
	}

	result := server.ioMetricsToProto(io)

	if result.ReadRate != 0.0 {
		t.Errorf("ReadRate: expected 0.0, got %f", result.ReadRate)
	}
	if result.WriteRate != 0.0 {
		t.Errorf("WriteRate: expected 0.0, got %f", result.WriteRate)
	}
	if len(result.DiskIo) != 1 {
		t.Fatalf("Expected 1 per-device metric, got %d", len(result.DiskIo))
	}
	if result.DiskIo[0].Utilization != 0.0 {
		t.Errorf("Device utilization: expected 0.0, got %f", result.DiskIo[0].Utilization)
	}
}
