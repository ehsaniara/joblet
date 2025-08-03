package values

import (
	"testing"
)

func TestNewCPUPercentage(t *testing.T) {
	tests := []struct {
		name    string
		percent int32
		wantErr bool
	}{
		{"zero cpu (unlimited)", 0, false},
		{"50% cpu", 50, false},
		{"100% cpu (1 core)", 100, false},
		{"200% cpu (2 cores)", 200, false},
		{"1000% cpu (10 cores)", 1000, false},
		{"negative cpu", -1, true},
		{"too high cpu", 10001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, err := NewCPUPercentage(tt.percent)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCPUPercentage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cpu.Value() != tt.percent {
				t.Errorf("NewCPUPercentage() = %v, want %v", cpu.Value(), tt.percent)
			}
		})
	}
}

func TestCPUPercentage_Cores(t *testing.T) {
	tests := []struct {
		name    string
		percent int32
		want    float32
	}{
		{"0% = 0 cores", 0, 0},
		{"50% = 0.5 cores", 50, 0.5},
		{"100% = 1 core", 100, 1.0},
		{"200% = 2 cores", 200, 2.0},
		{"150% = 1.5 cores", 150, 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, _ := NewCPUPercentage(tt.percent)
			if got := cpu.Cores(); got != tt.want {
				t.Errorf("CPUPercentage.Cores() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCPUCoreSet(t *testing.T) {
	tests := []struct {
		name      string
		spec      string
		wantCores []int
		wantCount int
		wantErr   bool
	}{
		{"empty spec", "", []int{}, 0, false},
		{"single core", "2", []int{2}, 1, false},
		{"core list", "0,2,4,6", []int{0, 2, 4, 6}, 4, false},
		{"core range", "0-3", []int{0, 1, 2, 3}, 4, false},
		{"invalid range format", "0-3-5", nil, 0, true},
		{"invalid range start>end", "3-0", nil, 0, true},
		{"invalid number", "abc", nil, 0, true},
		{"negative core", "-1", nil, 0, true},
		{"duplicate cores", "1,2,1", nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coreSet, err := ParseCPUCoreSet(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCPUCoreSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if coreSet.Count() != tt.wantCount {
				t.Errorf("ParseCPUCoreSet() count = %v, want %v", coreSet.Count(), tt.wantCount)
			}

			cores := coreSet.Cores()
			if len(cores) != len(tt.wantCores) {
				t.Errorf("ParseCPUCoreSet() cores = %v, want %v", cores, tt.wantCores)
				return
			}

			for i, core := range cores {
				if core != tt.wantCores[i] {
					t.Errorf("ParseCPUCoreSet() core[%d] = %v, want %v", i, core, tt.wantCores[i])
				}
			}
		})
	}
}

func TestCPUCoreSet_Contains(t *testing.T) {
	coreSet, _ := ParseCPUCoreSet("0,2,4,6")

	tests := []struct {
		core int
		want bool
	}{
		{0, true},
		{1, false},
		{2, true},
		{3, false},
		{4, true},
		{5, false},
		{6, true},
		{7, false},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.core+'0')), func(t *testing.T) {
			if got := coreSet.Contains(tt.core); got != tt.want {
				t.Errorf("CPUCoreSet.Contains(%d) = %v, want %v", tt.core, got, tt.want)
			}
		})
	}
}

func TestCPUCoreSet_Validate(t *testing.T) {
	tests := []struct {
		name           string
		spec           string
		availableCores int
		wantErr        bool
	}{
		{"valid cores", "0-3", 8, false},
		{"core out of range", "0-7", 4, true},
		{"single core out of range", "8", 8, true},
		{"empty spec", "", 8, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coreSet, _ := ParseCPUCoreSet(tt.spec)
			err := coreSet.Validate(tt.availableCores)
			if (err != nil) != tt.wantErr {
				t.Errorf("CPUCoreSet.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
