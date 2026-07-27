package values

import (
	"testing"
)

func TestNewMemorySize(t *testing.T) {
	tests := []struct {
		name    string
		bytes   int64
		wantErr bool
	}{
		{"zero memory (unlimited)", 0, false},
		{"1 byte", 1, false},
		{"1KB", 1024, false},
		{"1MB", 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"negative memory", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem, err := NewMemorySize(tt.bytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMemorySize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && mem.Bytes() != tt.bytes {
				t.Errorf("NewMemorySize() = %v, want %v", mem.Bytes(), tt.bytes)
			}
		})
	}
}

func TestMemorySize_Conversions(t *testing.T) {
	mem, _ := NewMemorySize(2 * 1024 * 1024 * 1024) // 2GB

	if got := mem.Megabytes(); got != 2048 {
		t.Errorf("MemorySize.Megabytes() = %v, want %v", got, 2048)
	}

	if got := mem.Gigabytes(); got != 2.0 {
		t.Errorf("MemorySize.Gigabytes() = %v, want %v", got, 2.0)
	}
}

func TestMemorySize_String(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0"},
		{"bytes", 512, "512 B"},
		{"kilobytes", 2048, "2.0 KB"},
		{"megabytes", 1536 * 1024, "1.5 MB"},
		{"gigabytes", 2 * 1024 * 1024 * 1024, "2.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem, _ := NewMemorySize(tt.bytes)
			if got := mem.String(); got != tt.want {
				t.Errorf("MemorySize.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemorySize_Arithmetic(t *testing.T) {
	mem1, _ := NewMemorySizeFromMB(512)
	mem2, _ := NewMemorySizeFromMB(256)

	// Test addition
	sum := mem1.Add(mem2)
	if sum.Megabytes() != 768 {
		t.Errorf("MemorySize.Add() = %v MB, want %v MB", sum.Megabytes(), 768)
	}

	// Test subtraction
	diff, err := mem1.Subtract(mem2)
	if err != nil {
		t.Errorf("MemorySize.Subtract() unexpected error: %v", err)
	}
	if diff.Megabytes() != 256 {
		t.Errorf("MemorySize.Subtract() = %v MB, want %v MB", diff.Megabytes(), 256)
	}

	// Test invalid subtraction
	_, err = mem2.Subtract(mem1)
	if err == nil {
		t.Error("MemorySize.Subtract() expected error for negative result")
	}
}

func TestMemorySize_Validate(t *testing.T) {
	minMem, _ := NewMemorySizeFromMB(64)
	maxMem, _ := NewMemorySizeFromMB(1024)

	tests := []struct {
		name    string
		memMB   int32
		wantErr bool
	}{
		{"within bounds", 512, false},
		{"at minimum", 64, false},
		{"at maximum", 1024, false},
		{"below minimum", 32, true},
		{"above maximum", 2048, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem, _ := NewMemorySizeFromMB(tt.memMB)
			err := mem.Validate(minMem, maxMem)
			if (err != nil) != tt.wantErr {
				t.Errorf("MemorySize.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
