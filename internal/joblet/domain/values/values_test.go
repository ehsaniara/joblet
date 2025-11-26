package values

import (
	"strings"
	"testing"
)

// ============================================================================
// Bandwidth Tests
// ============================================================================

func TestNewBandwidth(t *testing.T) {
	tests := []struct {
		name        string
		bytesPerSec int64
		wantErr     bool
	}{
		{"zero bandwidth", 0, false},
		{"positive bandwidth", 1024, false},
		{"large bandwidth", 1024 * 1024 * 1024, false},
		{"negative bandwidth", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bw, err := NewBandwidth(tt.bytesPerSec)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewBandwidth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && bw.BytesPerSecond() != tt.bytesPerSec {
				t.Errorf("BytesPerSecond() = %v, want %v", bw.BytesPerSecond(), tt.bytesPerSec)
			}
		})
	}
}

func TestParseBandwidth(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"empty string", "", 0, false},
		{"zero", "0", 0, false},
		{"bytes", "100B/s", 100, false},
		{"bytes no suffix", "100B", 100, false},
		{"kilobytes", "10KB/s", 10 * 1024, false},
		{"kilobytes short", "10K", 10 * 1024, false},
		{"megabytes", "5MB/s", 5 * 1024 * 1024, false},
		{"megabytes short", "5M", 5 * 1024 * 1024, false},
		{"gigabytes", "1GB/s", 1024 * 1024 * 1024, false},
		{"gigabytes short", "1G", 1024 * 1024 * 1024, false},
		{"fractional megabytes", "1.5MB/s", int64(1.5 * 1024 * 1024), false},
		{"lowercase suffix", "10mb/s", 10 * 1024 * 1024, false},
		{"uppercase S suffix", "10MB/S", 10 * 1024 * 1024, false},
		{"invalid unit", "10XB/s", 0, true},
		{"no number", "MB/s", 0, true},
		{"invalid number", "abcMB/s", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bw, err := ParseBandwidth(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBandwidth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && bw.BytesPerSecond() != tt.want {
				t.Errorf("BytesPerSecond() = %v, want %v", bw.BytesPerSecond(), tt.want)
			}
		})
	}
}

func TestBandwidth_IsUnlimited(t *testing.T) {
	bw, _ := NewBandwidth(0)
	if !bw.IsUnlimited() {
		t.Error("zero bandwidth should be unlimited")
	}

	bw, _ = NewBandwidth(100)
	if bw.IsUnlimited() {
		t.Error("non-zero bandwidth should not be unlimited")
	}
}

func TestBandwidth_String(t *testing.T) {
	tests := []struct {
		name        string
		bytesPerSec int64
		wantContain string
	}{
		{"unlimited", 0, "unlimited"},
		{"bytes", 512, "512 B/s"},
		{"kilobytes", 2 * 1024, "K"},
		{"megabytes", 5 * 1024 * 1024, "M"},
		{"gigabytes", 2 * 1024 * 1024 * 1024, "G"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bw, _ := NewBandwidth(tt.bytesPerSec)
			got := bw.String()
			if !strings.Contains(got, tt.wantContain) {
				t.Errorf("String() = %v, want to contain %v", got, tt.wantContain)
			}
		})
	}
}

func TestBandwidth_Validate(t *testing.T) {
	min, _ := NewBandwidth(100)
	max, _ := NewBandwidth(1000)

	tests := []struct {
		name    string
		bw      int64
		wantErr bool
	}{
		{"within range", 500, false},
		{"at min", 100, false},
		{"at max", 1000, false},
		{"below min", 50, true},
		{"above max", 1500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bw, _ := NewBandwidth(tt.bw)
			err := bw.Validate(min, max)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ============================================================================
// JobID Tests
// ============================================================================

func TestNewJobID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid UUID", "12345678-abcd-1234-abcd-123456789012", false},
		{"valid short ID", "12345678", false},
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"too short", "1234567", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jid, err := NewJobID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewJobID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && jid.String() != tt.id {
				t.Errorf("String() = %v, want %v", jid.String(), tt.id)
			}
		})
	}
}

func TestMustJobID(t *testing.T) {
	jid := MustJobID("test-job-id")
	if jid.Value() != "test-job-id" {
		t.Errorf("MustJobID() Value() = %v, want test-job-id", jid.Value())
	}
}

func TestJobID_IsEmpty(t *testing.T) {
	jid := MustJobID("")
	if !jid.IsEmpty() {
		t.Error("empty JobID should return true for IsEmpty()")
	}

	jid = MustJobID("test-id")
	if jid.IsEmpty() {
		t.Error("non-empty JobID should return false for IsEmpty()")
	}
}

// ============================================================================
// ProcessID Tests
// ============================================================================

func TestNewProcessID(t *testing.T) {
	tests := []struct {
		name    string
		pid     int32
		wantErr bool
	}{
		{"valid PID", 1234, false},
		{"PID 1", 1, false},
		{"zero PID", 0, true},
		{"negative PID", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pid, err := NewProcessID(tt.pid)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProcessID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && pid.Value() != tt.pid {
				t.Errorf("Value() = %v, want %v", pid.Value(), tt.pid)
			}
		})
	}
}

func TestProcessID_String(t *testing.T) {
	pid, _ := NewProcessID(1234)
	if pid.String() != "1234" {
		t.Errorf("String() = %v, want 1234", pid.String())
	}
}

func TestProcessID_IsValid(t *testing.T) {
	pid, _ := NewProcessID(1234)
	if !pid.IsValid() {
		t.Error("valid PID should return true for IsValid()")
	}

	// Zero PID (default value) should not be valid
	zeroPID := ProcessID{}
	if zeroPID.IsValid() {
		t.Error("zero PID should return false for IsValid()")
	}
}

// ============================================================================
// NetworkName Tests
// ============================================================================

func TestNewNetworkName(t *testing.T) {
	tests := []struct {
		name    string
		network string
		wantErr bool
	}{
		{"valid name", "my-network", false},
		{"valid single char", "n", false},
		{"valid with numbers", "network123", false},
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"starts with hyphen", "-network", true},
		{"ends with hyphen", "network-", true},
		{"invalid chars", "my_network", true},
		{"too long", strings.Repeat("a", 64), true},
		{"max length", strings.Repeat("a", 63), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nn, err := NewNetworkName(tt.network)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewNetworkName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && nn.Value() != tt.network {
				t.Errorf("Value() = %v, want %v", nn.Value(), tt.network)
			}
		})
	}
}

func TestNetworkName_IsIsolated(t *testing.T) {
	nn, _ := NewNetworkName("isolated")
	if !nn.IsIsolated() {
		t.Error("'isolated' network should return true for IsIsolated()")
	}

	nn, _ = NewNetworkName("bridge")
	if nn.IsIsolated() {
		t.Error("'bridge' network should return false for IsIsolated()")
	}
}

// ============================================================================
// VolumeName Tests
// ============================================================================

func TestNewVolumeName(t *testing.T) {
	tests := []struct {
		name    string
		volume  string
		wantErr bool
	}{
		{"valid name", "my-volume", false},
		{"valid with underscore", "my_volume", false},
		{"valid with dot", "my.volume", false},
		{"valid with numbers", "volume123", false},
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"starts with special", "-volume", true},
		{"invalid chars", "my@volume", true},
		{"too long", strings.Repeat("a", 129), true},
		{"max length", strings.Repeat("a", 128), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vn, err := NewVolumeName(tt.volume)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewVolumeName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && vn.Value() != tt.volume {
				t.Errorf("Value() = %v, want %v", vn.Value(), tt.volume)
			}
		})
	}
}

// ============================================================================
// RuntimeSpec Tests
// ============================================================================

func TestNewRuntimeSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		wantErr bool
	}{
		{"valid spec", "python-3.11-ml", false},
		{"empty spec", "", false},
		{"whitespace only", "   ", false},
		{"too long", strings.Repeat("a", 257), true},
		{"max length", strings.Repeat("a", 256), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs, err := NewRuntimeSpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRuntimeSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && rs.Value() != strings.TrimSpace(tt.spec) {
				t.Errorf("Value() = %v, want %v", rs.Value(), strings.TrimSpace(tt.spec))
			}
		})
	}
}

func TestRuntimeSpec_IsEmpty(t *testing.T) {
	rs, _ := NewRuntimeSpec("")
	if !rs.IsEmpty() {
		t.Error("empty RuntimeSpec should return true for IsEmpty()")
	}

	rs, _ = NewRuntimeSpec("python-3.11")
	if rs.IsEmpty() {
		t.Error("non-empty RuntimeSpec should return false for IsEmpty()")
	}
}

func TestRuntimeSpec_Language(t *testing.T) {
	tests := []struct {
		spec string
		want string
	}{
		{"python:3.11", "python"},
		{"node:18", "node"},
		{"python", "python"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			rs, _ := NewRuntimeSpec(tt.spec)
			if got := rs.Language(); got != tt.want {
				t.Errorf("Language() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuntimeSpec_Version(t *testing.T) {
	tests := []struct {
		spec string
		want string
	}{
		{"python:3.11", "3.11"},
		{"node:18-alpine", "18-alpine"},
		{"python", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			rs, _ := NewRuntimeSpec(tt.spec)
			if got := rs.Version(); got != tt.want {
				t.Errorf("Version() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// Path Tests
// ============================================================================

func TestNewPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid absolute path", "/usr/local/bin", false},
		{"valid relative path", "local/bin", false},
		{"empty", "", true},
		{"whitespace only", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && p.IsEmpty() {
				t.Error("non-empty path should not return true for IsEmpty()")
			}
		})
	}
}

func TestNewAbsolutePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid absolute path", "/usr/local/bin", false},
		{"relative path", "local/bin", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewAbsolutePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAbsolutePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !p.IsAbsolute() {
				t.Error("NewAbsolutePath should return absolute path")
			}
		})
	}
}

func TestPath_Dir(t *testing.T) {
	p, _ := NewPath("/usr/local/bin/file.txt")
	dir := p.Dir()
	if dir.Value() != "/usr/local/bin" {
		t.Errorf("Dir() = %v, want /usr/local/bin", dir.Value())
	}
}

func TestPath_Base(t *testing.T) {
	p, _ := NewPath("/usr/local/bin/file.txt")
	if p.Base() != "file.txt" {
		t.Errorf("Base() = %v, want file.txt", p.Base())
	}
}

func TestPath_Join(t *testing.T) {
	p, _ := NewPath("/usr/local")
	joined := p.Join("bin", "file.txt")
	if joined.Value() != "/usr/local/bin/file.txt" {
		t.Errorf("Join() = %v, want /usr/local/bin/file.txt", joined.Value())
	}
}

// ============================================================================
// CgroupPath Tests
// ============================================================================

func TestNewCgroupPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid cgroup v2 path", "/sys/fs/cgroup/joblet.slice", false},
		{"valid cgroup2 path", "/sys/fs/cgroup2/joblet.slice", false},
		{"invalid prefix", "/var/cgroup/joblet", true},
		{"relative path", "sys/fs/cgroup/joblet", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp, err := NewCgroupPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCgroupPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cp.Value() == "" {
				t.Error("valid CgroupPath should not have empty value")
			}
		})
	}
}

func TestCgroupPath_IsV1(t *testing.T) {
	cp, _ := NewCgroupPath("/sys/fs/cgroup/memory/joblet")
	if !cp.IsV1() {
		t.Error("/sys/fs/cgroup/ path should be V1")
	}
}

func TestCgroupPath_IsV2(t *testing.T) {
	cp, _ := NewCgroupPath("/sys/fs/cgroup2/joblet.slice")
	if !cp.IsV2() {
		t.Error("/sys/fs/cgroup2/ path should be V2")
	}
}

// ============================================================================
// WorkspacePath Tests
// ============================================================================

func TestNewWorkspacePath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		jobID    string
		wantErr  bool
	}{
		{"valid workspace", "/opt/joblet/jobs", "job-123", false},
		{"empty base path", "", "job-123", true},
		{"empty job ID", "/opt/joblet/jobs", "", true},
		{"whitespace base path", "   ", "job-123", true},
		{"whitespace job ID", "/opt/joblet/jobs", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wp, err := NewWorkspacePath(tt.basePath, tt.jobID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewWorkspacePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				expected := tt.basePath + "/" + tt.jobID + "/work"
				if wp.Value() != expected {
					t.Errorf("Value() = %v, want %v", wp.Value(), expected)
				}
			}
		})
	}
}

func TestWorkspacePath_JobDir(t *testing.T) {
	wp, _ := NewWorkspacePath("/opt/joblet/jobs", "job-123")
	jobDir := wp.JobDir()
	if jobDir.Value() != "/opt/joblet/jobs/job-123" {
		t.Errorf("JobDir() = %v, want /opt/joblet/jobs/job-123", jobDir.Value())
	}
}

// ============================================================================
// VolumeNames Tests
// ============================================================================

func TestNewVolumeNames(t *testing.T) {
	tests := []struct {
		name    string
		names   []string
		wantErr bool
	}{
		{"valid volumes", []string{"vol1", "vol2"}, false},
		{"empty slice", []string{}, false},
		{"invalid volume name", []string{"vol1", "-invalid"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vn, err := NewVolumeNames(tt.names)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewVolumeNames() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && vn.Count() != len(tt.names) {
				t.Errorf("Count() = %v, want %v", vn.Count(), len(tt.names))
			}
		})
	}
}

func TestVolumeNames_IsEmpty(t *testing.T) {
	vn, _ := NewVolumeNames([]string{})
	if !vn.IsEmpty() {
		t.Error("empty VolumeNames should return true for IsEmpty()")
	}

	vn, _ = NewVolumeNames([]string{"vol1"})
	if vn.IsEmpty() {
		t.Error("non-empty VolumeNames should return false for IsEmpty()")
	}
}

func TestVolumeNames_ToStringSlice(t *testing.T) {
	vn, _ := NewVolumeNames([]string{"vol1", "vol2"})
	slice := vn.ToStringSlice()
	if len(slice) != 2 {
		t.Errorf("ToStringSlice() length = %v, want 2", len(slice))
	}
}

func TestVolumeNames_Validate(t *testing.T) {
	vn, _ := NewVolumeNames([]string{"vol1", "vol2"})
	if err := vn.Validate(); err != nil {
		t.Errorf("Validate() should pass for unique volumes: %v", err)
	}

	// Create volume names with duplicates manually
	duplicateVols := VolumeNames{
		volumes: []VolumeName{
			{value: "vol1"},
			{value: "vol1"},
		},
	}
	if err := duplicateVols.Validate(); err == nil {
		t.Error("Validate() should fail for duplicate volumes")
	}
}

// ============================================================================
// Environment Tests
// ============================================================================

func TestNewEnvironment(t *testing.T) {
	vars := map[string]string{"KEY1": "value1", "KEY2": "value2"}
	env := NewEnvironment(vars)

	if env.Count() != 2 {
		t.Errorf("Count() = %v, want 2", env.Count())
	}

	// Verify it's a copy
	vars["KEY3"] = "value3"
	if env.Count() != 2 {
		t.Error("NewEnvironment should create a copy, not reference")
	}
}

func TestEmptyEnvironment(t *testing.T) {
	env := EmptyEnvironment()
	if !env.IsEmpty() {
		t.Error("EmptyEnvironment() should return empty environment")
	}
}

func TestEnvironment_ToMap(t *testing.T) {
	vars := map[string]string{"KEY1": "value1"}
	env := NewEnvironment(vars)
	result := env.ToMap()

	if result["KEY1"] != "value1" {
		t.Errorf("ToMap() KEY1 = %v, want value1", result["KEY1"])
	}

	// Verify it's a copy
	result["KEY2"] = "value2"
	if env.Count() != 1 {
		t.Error("ToMap() should return a copy")
	}
}

func TestEnvironment_ToSlice(t *testing.T) {
	vars := map[string]string{"KEY1": "value1", "KEY2": "value2"}
	env := NewEnvironment(vars)
	slice := env.ToSlice()

	if len(slice) != 2 {
		t.Errorf("ToSlice() length = %v, want 2", len(slice))
	}

	// Verify format
	found := false
	for _, s := range slice {
		if s == "KEY1=value1" || s == "KEY2=value2" {
			found = true
		}
	}
	if !found {
		t.Error("ToSlice() should contain KEY=VALUE formatted strings")
	}
}
