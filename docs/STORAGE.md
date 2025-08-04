# Joblet Storage Layer Design

## Overview

The Joblet storage layer provides a comprehensive solution for managing persistent and temporary storage in isolated job environments. It supports multiple storage backends, enforces disk quotas, and ensures complete isolation between jobs while maintaining performance and security.

## Architecture

### Core Components

```
Storage Layer
├── Volume Management
│   ├── Volume Types (filesystem, memory)
│   ├── Volume Lifecycle (create, mount, unmount, delete)
│   └── Volume Store (state management)
├── Filesystem Isolation
│   ├── Chroot Environments
│   ├── Mount Namespaces
│   └── Bind Mount Management
├── Disk Quota Management
│   ├── Default Work Directory (1MB tmpfs)
│   ├── Volume Size Limits
│   └── I/O Bandwidth Throttling
└── Storage APIs
    ├── gRPC Volume Service
    ├── CLI Commands
    └── Internal Interfaces
```

## Volume Management

### Volume Types

#### 1. Filesystem Volumes
- **Purpose**: Persistent storage that survives job restarts and system reboots
- **Implementation**: Directory-based storage on host filesystem
- **Location**: `/var/lib/joblet/volumes/<volume-id>`
- **Features**:
  - Persistent across job executions
  - Survives daemon restarts
  - Suitable for data processing workflows
  - Backed by host filesystem (ext4, xfs, etc.)

#### 2. Memory Volumes
- **Purpose**: High-performance temporary storage
- **Implementation**: tmpfs-based in-memory filesystem
- **Location**: Mounted at `/var/lib/joblet/volumes/<volume-id>` (tmpfs)
- **Features**:
  - Cleared after job completion
  - Ultra-fast I/O operations
  - No disk persistence
  - Ideal for temporary data and caches

### Volume Lifecycle

```go
// Volume creation flow
1. Client: rnx volume create mydata --size=1GB --type=filesystem
2. Server: Validates request and size constraints
3. VolumeManager: Creates volume directory/tmpfs
4. VolumeStore: Records volume metadata in memory
5. Server: Returns volume ID to client

// Volume mounting flow
1. Job execution request includes volume IDs
2. JobExecutor: Validates volume access permissions
3. VolumeManager: Prepares mount points
4. Isolation layer: Bind mounts volumes into job namespace
5. Job: Accesses volumes at /volumes/<name>

// Volume cleanup flow
1. Job completion triggers unmount
2. VolumeManager: Unmounts volumes from job namespace
3. Memory volumes: Data cleared automatically
4. Filesystem volumes: Data persists for next use
```

### Volume Store Implementation

```go
type VolumeStore struct {
    mu      sync.RWMutex
    volumes map[string]*domain.Volume
}

type Volume struct {
    ID          string
    Name        string
    Type        VolumeType // FILESYSTEM or MEMORY
    Size        int64      // Size in bytes
    Path        string     // Host path
    MountPath   string     // Path inside container
    CreatedAt   time.Time
    LastUsed    time.Time
    InUse       bool
    JobID       string     // Current job using volume
}
```

## Filesystem Isolation

### Isolation Layers

#### 1. Mount Namespace Isolation
- Each job runs in its own mount namespace
- Prevents jobs from seeing host filesystem mounts
- Enables per-job custom mount configurations
- Implemented using Linux `CLONE_NEWNS` flag

#### 2. Chroot Isolation
- Jobs execute within chroot jail at `/opt/joblet/jobs/<job-id>`
- Minimal root filesystem with essential binaries
- Prevents directory traversal attacks
- Combined with pivot_root for stronger isolation

#### 3. Bind Mount Management
```go
// Standard job filesystem layout
/opt/joblet/jobs/<job-id>/
├── bin/          # Essential binaries (busybox)
├── lib/          # Required libraries
├── lib64/        # 64-bit libraries
├── etc/          # Minimal configuration
├── proc/         # Process information (read-only)
├── dev/          # Device files (minimal set)
├── tmp/          # Temporary space
├── work/         # Job workspace (1MB default)
└── volumes/      # Mounted volumes
├── data/         # Example: filesystem volume
└── cache/        # Example: memory volume

```

### Mount Security

#### Path Traversal Protection
```go
func validatePath(base, target string) error {
    cleaned := filepath.Clean(target)
    abs, err := filepath.Abs(filepath.Join(base, cleaned))
    if err != nil {
        return err
    }
    if !strings.HasPrefix(abs, base) {
        return ErrPathTraversal
    }
    return nil
}
```

#### Read-Only System Mounts
- `/proc`: Read-only, filtered view
- `/sys`: Not mounted (security)
- System binaries: Read-only bind mounts
- Libraries: Read-only access

## Disk Quota Management

### Default Quotas

#### Work Directory (No Volumes)
- **Size**: 1MB tmpfs
- **Purpose**: Minimal runtime storage
- **Implementation**: tmpfs with size=1m
- **Rationale**: Prevents disk exhaustion from misconfigured jobs

#### Volume Quotas
- **Filesystem volumes**: Size limit enforced via disk quota or directory size monitoring
- **Memory volumes**: tmpfs size parameter
- **Enforcement**: Kernel-level (tmpfs) or application-level (filesystem)

### I/O Bandwidth Throttling

```go
// cgroup v2 I/O limits
type IOLimits struct {
    ReadBPS  uint64 // Read bytes per second
    WriteBPS uint64 // Write bytes per second
    ReadIOPS uint64 // Read operations per second
    WriteIOPS uint64 // Write operations per second
}

// Applied via io.max cgroup controller
// Example: "8:0 rbps=10485760 wbps=10485760"
```

## Storage APIs

### gRPC Volume Service

```protobuf
service VolumeService {
    // Volume lifecycle operations
    rpc CreateVolume(CreateVolumeRequest) returns (CreateVolumeResponse);
    rpc ListVolumes(ListVolumesRequest) returns (ListVolumesResponse);
    rpc GetVolume(GetVolumeRequest) returns (GetVolumeResponse);
    rpc DeleteVolume(DeleteVolumeRequest) returns (DeleteVolumeResponse);
    
    // Volume usage operations
    rpc AttachVolume(AttachVolumeRequest) returns (AttachVolumeResponse);
    rpc DetachVolume(DetachVolumeRequest) returns (DetachVolumeResponse);
}

message CreateVolumeRequest {
    string name = 1;
    VolumeType type = 2;
    int64 size_bytes = 3;
    map<string, string> labels = 4;
}

message Volume {
    string id = 1;
    string name = 2;
    VolumeType type = 3;
    int64 size_bytes = 4;
    string status = 5;
    google.protobuf.Timestamp created_at = 6;
    google.protobuf.Timestamp last_used = 7;
    string current_job_id = 8;
}
```

### CLI Commands

```bash
# Volume management commands
rnx volume create <name> [options]
  --size=SIZE       Volume size (e.g., 1GB, 512MB)
  --type=TYPE       Volume type: filesystem|memory
  --label=KEY=VAL   Metadata labels

rnx volume list [options]
  --filter=KEY=VAL  Filter by label
  --format=FORMAT   Output format: table|json|yaml

rnx volume inspect <name>
  Shows detailed volume information

rnx volume remove <name> [options]
  --force           Remove even if in use

# Job execution with volumes
rnx run --volume=data:/data --volume=cache:/cache <command>
```

### Internal Interfaces

```go
// VolumeManager interface
type VolumeManager interface {
    CreateVolume(ctx context.Context, req *CreateVolumeRequest) (*Volume, error)
    GetVolume(ctx context.Context, id string) (*Volume, error)
    ListVolumes(ctx context.Context, filter *VolumeFilter) ([]*Volume, error)
    DeleteVolume(ctx context.Context, id string) error
    
    // Job integration
    PrepareVolumes(ctx context.Context, jobID string, volumeIDs []string) error
    CleanupVolumes(ctx context.Context, jobID string) error
}

// StorageProvider interface (for extensibility)
type StorageProvider interface {
    Create(ctx context.Context, volume *Volume) error
    Delete(ctx context.Context, volume *Volume) error
    Mount(ctx context.Context, volume *Volume, target string) error
    Unmount(ctx context.Context, volume *Volume) error
    GetUsage(ctx context.Context, volume *Volume) (*UsageInfo, error)
}
```

## Implementation Details

### Volume Creation Process

```go
func (vm *VolumeManager) CreateVolume(ctx context.Context, req *CreateVolumeRequest) (*Volume, error) {
    // 1. Validate request
    if err := validateVolumeRequest(req); err != nil {
        return nil, err
    }
    
    // 2. Generate unique ID
    volumeID := generateVolumeID()
    volumePath := filepath.Join(vm.baseDir, volumeID)
    
    // 3. Create volume based on type
    switch req.Type {
    case VolumeTypeFilesystem:
        if err := os.MkdirAll(volumePath, 0755); err != nil {
            return nil, err
        }
        // Set up disk quota if supported
        if vm.quotaEnabled {
            setDiskQuota(volumePath, req.SizeBytes)
        }
        
    case VolumeTypeMemory:
        // Memory volumes created on-demand during mount
        // Just validate size doesn't exceed system limits
        if req.SizeBytes > vm.maxMemoryVolumeSize {
            return nil, ErrVolumeTooLarge
        }
    }
    
    // 4. Create volume record
    volume := &Volume{
        ID:        volumeID,
        Name:      req.Name,
        Type:      req.Type,
        Size:      req.SizeBytes,
        Path:      volumePath,
        CreatedAt: time.Now(),
        Status:    VolumeStatusAvailable,
    }
    
    // 5. Store in volume store
    if err := vm.store.Add(volume); err != nil {
        // Cleanup on failure
        os.RemoveAll(volumePath)
        return nil, err
    }
    
    return volume, nil
}
```

### Job Volume Mounting

```go
func (je *JobExecutor) mountVolumes(job *Job) error {
    for _, volumeMount := range job.VolumeMounts {
        volume, err := je.volumeManager.GetVolume(volumeMount.VolumeID)
        if err != nil {
            return fmt.Errorf("volume %s not found: %w", volumeMount.VolumeID, err)
        }
        
        // Create mount point in job filesystem
        targetPath := filepath.Join(job.RootFS, "volumes", volumeMount.MountPath)
        if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
            return err
        }
        
        switch volume.Type {
        case VolumeTypeFilesystem:
            // Bind mount filesystem volume
            if err := mount.BindMount(volume.Path, targetPath, false); err != nil {
                return err
            }
            
        case VolumeTypeMemory:
            // Create tmpfs mount
            opts := fmt.Sprintf("size=%d", volume.Size)
            if err := mount.TmpfsMount(targetPath, opts); err != nil {
                return err
            }
        }
        
        // Record mount for cleanup
        job.mounts = append(job.mounts, targetPath)
    }
    return nil
}
```

### Cleanup and Lifecycle Management

```go
func (vm *VolumeManager) cleanupUnusedVolumes(ctx context.Context) {
    ticker := time.NewTicker(vm.cleanupInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            volumes, _ := vm.store.List(&VolumeFilter{
                Status: VolumeStatusAvailable,
            })
            
            for _, volume := range volumes {
                // Clean up memory volumes not used recently
                if volume.Type == VolumeTypeMemory {
                    if time.Since(volume.LastUsed) > vm.memoryVolumeTimeout {
                        vm.DeleteVolume(ctx, volume.ID)
                    }
                }
                
                // Clean up orphaned mounts
                if volume.InUse && volume.JobID != "" {
                    if !vm.jobStore.Exists(volume.JobID) {
                        vm.DetachVolume(ctx, volume.ID)
                    }
                }
            }
        }
    }
}
```

## Security Considerations

### Access Control
- Volume access tied to job execution permissions
- No cross-job volume access without explicit sharing
- Volume names must be unique per user/namespace

### Resource Limits
- Maximum volume size limits prevent resource exhaustion
- Total volume count limits per user
- I/O bandwidth throttling prevents DoS

### Data Isolation
- Each volume mounted in isolated namespace
- No direct host filesystem access
- Encrypted volume support (future enhancement)

## Performance Optimization

### Caching Strategy
- Frequently used volumes kept mounted
- LRU eviction for memory volumes
- Metadata cached in memory

### I/O Optimization
- Direct I/O for large files
- Buffered I/O for small files
- Async cleanup operations

### Monitoring
- Volume usage metrics
- Mount/unmount latency tracking
- I/O throughput monitoring

## Future Enhancements

### Planned Features
1. **Network Storage Support**
   - NFS volume driver
   - S3-compatible object storage
   - Distributed storage backends

2. **Advanced Quota Management**
   - Project quotas
   - User quotas
   - Soft/hard limit support

3. **Snapshot and Backup**
   - Volume snapshots
   - Incremental backups
   - Point-in-time recovery

4. **Encryption**
   - At-rest encryption for filesystem volumes
   - Encrypted memory volumes
   - Key management integration

5. **Volume Sharing**
   - Read-only volume sharing between jobs
   - Copy-on-write volumes
   - Volume cloning

## Testing Strategy

### Unit Tests
- Volume CRUD operations
- Mount/unmount logic
- Quota enforcement
- Path validation

### Integration Tests
- End-to-end volume lifecycle
- Job execution with volumes
- Concurrent volume access
- Failure recovery

### Performance Tests
- Volume creation latency
- Mount/unmount performance
- I/O throughput benchmarks
- Concurrent operation stress tests

## Appendix: Volume Size Parsing

```go
// Supported size formats
"512"    -> 512 bytes
"10KB"   -> 10 * 1024 bytes
"5MB"    -> 5 * 1024 * 1024 bytes
"1GB"    -> 1 * 1024 * 1024 * 1024 bytes
"100Mi"  -> 100 * 1024 * 1024 bytes (IEC format)
"2Gi"    -> 2 * 1024 * 1024 * 1024 bytes (IEC format)
```